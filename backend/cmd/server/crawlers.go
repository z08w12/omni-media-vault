package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives/scriptcrawler"
)

func (a *App) scheduleScriptCrawlerCrawl(ctx context.Context, driveID string) bool {
	if a.driveHasActiveWork(driveID) {
		log.Printf("[scriptcrawler] drive=%s has active work, skip duplicate crawl request", driveID)
		return false
	}
	taskCtx, done, admitted := a.registerDriveTaskContext(ctx, driveID, driveTaskScopeScan)
	if !admitted {
		log.Printf("[scriptcrawler] drive=%s configuration update in progress, reject crawl", driveID)
		return false
	}
	if !a.beginDriveScanOrCrawl(driveID) {
		done()
		log.Printf("[scriptcrawler] drive=%s already queued or running, skip duplicate crawl request", driveID)
		return false
	}

	go func() {
		defer func() {
			a.endDriveScanOrCrawl(driveID)
			done()
		}()
		if a.runScriptCrawlerCrawlWithTaskContext(taskCtx, driveID) {
			a.runCrawlerPostCrawl(taskCtx, driveID)
		}
	}()
	return true
}

func (a *App) runScriptCrawlerCrawl(ctx context.Context, driveID string) {
	taskCtx, done, admitted := a.registerDriveTaskContext(ctx, driveID, driveTaskScopeScan)
	if !admitted {
		log.Printf("[scriptcrawler] drive=%s configuration update in progress, reject direct crawl", driveID)
		return
	}
	defer done()
	if !a.beginDriveScanOrCrawl(driveID) {
		log.Printf("[scriptcrawler] drive=%s already queued or running, skip direct crawl", driveID)
		return
	}
	defer a.endDriveScanOrCrawl(driveID)
	a.runScriptCrawlerCrawlWithTaskContext(taskCtx, driveID)
}

func (a *App) runScriptCrawlerCrawlWithTaskContext(ctx context.Context, driveID string) bool {
	if err := ctx.Err(); err != nil {
		log.Printf("[scriptcrawler] drive=%s crawl canceled before start: %v", driveID, err)
		return false
	}
	a.mu.Lock()
	c := a.scriptCrawlers[driveID]
	a.mu.Unlock()
	if c == nil {
		if err := a.ensureDriveAttached(ctx, driveID); err != nil {
			log.Printf("[scriptcrawler] drive=%s attach failed: %v", driveID, err)
			return false
		}
		a.mu.Lock()
		c = a.scriptCrawlers[driveID]
		a.mu.Unlock()
		if c == nil {
			log.Printf("[scriptcrawler] drive=%s crawler not attached", driveID)
			return false
		}
	}

	d, err := a.activeDriveConfig(ctx, driveID)
	if err != nil || d == nil {
		log.Printf("[scriptcrawler] drive=%s active configuration lookup failed: %v", driveID, err)
		return false
	}
	targetNew := crawlerIntCred(d, "target_new", scriptcrawler.DefaultTargetNew)
	if targetNew <= 0 {
		targetNew = scriptcrawler.DefaultTargetNew
	}

	log.Printf("[scriptcrawler] drive=%s start crawl target_new=%d", driveID, targetNew)
	res, runErr := c.RunOnce(ctx, targetNew)
	if runErr != nil {
		log.Printf("[scriptcrawler] drive=%s crawl failed: %v", driveID, runErr)
	} else if res != nil {
		log.Printf("[scriptcrawler] drive=%s crawl done target=%d candidate_budget=%d total=%d new=%d skipped=%d failed=%d seen_snapshot=%d",
			driveID, res.TargetNew, res.CandidateBudget, res.TotalEntries, res.NewVideos, res.Skipped, res.Failed, res.SeenSnapshot)
	}

	if err := a.updateScriptCrawlerRunState(ctx, driveID, runErr); err != nil {
		log.Printf("[scriptcrawler] drive=%s update last_crawl_at: %v", driveID, err)
	}
	if err := ctx.Err(); err != nil {
		log.Printf("[scriptcrawler] drive=%s crawl canceled after run: %v", driveID, err)
		return false
	}

	a.mu.Lock()
	worker := a.workers[driveID]
	thumbWorker := a.thumbWorkers[driveID]
	fingerprintWorker := a.fingerprintWorkers[driveID]
	a.mu.Unlock()
	a.enqueueFingerprintBackfill(ctx, driveID, fingerprintWorker)
	a.enqueueDriveGeneration(ctx, driveID, worker, thumbWorker)
	return runErr == nil
}

func (a *App) updateScriptCrawlerRunState(ctx context.Context, driveID string, runErr error) error {
	status, lastError := "ok", ""
	if runErr != nil {
		status, lastError = "error", runErr.Error()
	}
	return a.cat.UpdateDriveRuntimeState(ctx, driveID, scriptcrawler.Kind, status, lastError, map[string]string{
		"last_crawl_at": strconv.FormatInt(time.Now().Unix(), 10),
	})
}

func (a *App) runCrawlerPostCrawl(ctx context.Context, driveID string) {
	if err := ctx.Err(); err != nil {
		return
	}
	log.Printf("[scriptcrawler] drive=%s waiting for generation queues before post-crawl completion", driveID)
	if err := a.waitDriveGenerationQueuesIdle(ctx, driveID); err != nil {
		log.Printf("[scriptcrawler] drive=%s post-crawl wait canceled: %v", driveID, err)
		return
	}
	if err := a.restoreScriptCrawlerVideos(ctx, driveID); err != nil {
		log.Printf("[scriptcrawler] drive=%s post-crawl restore: %v", driveID, err)
	}
}

func (a *App) restoreScriptCrawlerVideos(ctx context.Context, driveID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	taskCtx, done, admitted := a.registerDriveTaskContext(ctx, driveID, 0)
	if !admitted {
		return fmt.Errorf("restore crawler drive %s: configuration update in progress", driveID)
	}
	defer done()
	ctx = taskCtx
	requests, err := a.cat.ListCrawlerRestoreRequests(ctx, driveID)
	if err != nil || len(requests) == 0 {
		return err
	}
	if err := a.ensureDriveAttached(ctx, driveID); err != nil {
		return err
	}
	a.mu.Lock()
	crawler := a.scriptCrawlers[driveID]
	a.mu.Unlock()
	if crawler == nil {
		return nil
	}
	restored, err := crawler.RestoreRequestedVideos(ctx)
	if restored > 0 {
		a.mu.Lock()
		worker := a.workers[driveID]
		thumbWorker := a.thumbWorkers[driveID]
		fingerprintWorker := a.fingerprintWorkers[driveID]
		a.mu.Unlock()
		a.enqueueFingerprintBackfill(ctx, driveID, fingerprintWorker)
		a.enqueueDriveGeneration(ctx, driveID, worker, thumbWorker)
	}
	return err
}

// crawlerIntCred 解析 credentials 中的整数字段，缺省时返回 def。
func crawlerIntCred(d *catalog.Drive, key string, def int) int {
	if d == nil || d.Credentials == nil {
		return def
	}
	raw := strings.TrimSpace(d.Credentials[key])
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
