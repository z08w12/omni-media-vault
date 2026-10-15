package catalog

import (
	"context"
)

// BuiltinTagsEnabled reports whether the built-in tag pack is enabled.
// Built-in tags are deprecated and permanently disabled.
func (c *Catalog) BuiltinTagsEnabled(ctx context.Context) (bool, error) {
	return false, nil
}

// SetBuiltinTagsEnabled is a no-op / disabler since built-in tags have been removed.
func (c *Catalog) SetBuiltinTagsEnabled(ctx context.Context, enabled bool) (bool, error) {
	if enabled {
		return false, ErrBuiltinTagsDisabled
	}
	return false, nil
}
