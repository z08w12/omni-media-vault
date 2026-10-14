import { type ReactNode } from "react";
import { useDevice } from "./DeviceContext";

type AdaptiveViewProps = {
  desktop: ReactNode;
  mobile: ReactNode;
};

/**
 * Declarative dual-track view container:
 * Automatically renders desktop or mobile sub-tree based on DeviceContext.
 */
export function AdaptiveView({ desktop, mobile }: AdaptiveViewProps) {
  const { isMobile } = useDevice();
  return <>{isMobile ? mobile : desktop}</>;
}
