import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router";
import App from "./App";
import { ToastProvider } from "@/components/ToastContext";
import { AuthProvider } from "./admin/AuthContext";
import { DeviceProvider } from "./platform";
import { syncThemeFromServer } from "./lib/theme";
import { initializeListingScrollRestore } from "./lib/listingScrollRestore";
import { initializeControlFocus } from "./lib/controlFocus";
import { registerServiceWorker } from "./pwa/registerServiceWorker";

import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/layout.css";
import "./styles/navigation.css";
import "./styles/search.css";
import "./styles/video-card.css";
import "./styles/video-detail.css";
import "./styles/video-card-interactions.css";
import "./styles/shared-state.css";
import "./styles/mobile-tabbar.css";
import "./styles/desktop-video.css";
import "./styles/mobile-video.css";
import "./styles/tag-album.css";
import "./styles/masonry.css";

// 启动时和服务端对齐一次。失败也无所谓，index.html 已经从 localStorage
// 设了一个合理初值。这里不 await，挂载和拉主题并行。
syncThemeFromServer();

initializeListingScrollRestore(window);
initializeControlFocus(document);
registerServiceWorker();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <AuthProvider>
          <DeviceProvider>
            <App />
          </DeviceProvider>
        </AuthProvider>
      </ToastProvider>
    </BrowserRouter>
  </React.StrictMode>
);
