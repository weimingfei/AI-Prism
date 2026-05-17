import { useCallback, useState } from "react";
import { type MediaError } from "@/lib/media";

export type CameraErrorCopy = {
  title: string;
  description: string;
} | null;

const getMediaErrorCopy = (error: MediaError | null): CameraErrorCopy => {
  if (!error) return null;
  switch (error.kind) {
    case "permission_denied":
      return {
        title: "无法使用摄像头",
        description: "你还没有授予摄像头权限。请在浏览器地址栏或系统设置中允许访问后重试。",
      };
    case "not_found":
      return {
        title: "未检测到摄像头",
        description: "请检查设备连接，或确认系统摄像头设置是否可用。",
      };
    case "not_readable":
      return {
        title: "摄像头暂时不可用",
        description: "摄像头可能正在被其他应用占用，请关闭后重试。",
      };
    case "overconstrained":
      return {
        title: "摄像头参数不支持",
        description: "当前设备不支持所需摄像头配置，请更换设备或稍后重试。",
      };
    case "not_supported":
      return {
        title: "当前环境不支持摄像头",
        description: "请使用 HTTPS 或 localhost 访问页面后再开启摄像头。",
      };
    default:
      return {
        title: "摄像头初始化失败",
        description: "请稍后重试，或更换浏览器再试。",
      };
  }
};

export function useInterviewCameraState() {
  const [isCameraOpen, setIsCameraOpen] = useState(false);
  const [isCameraExpanded, setIsCameraExpanded] = useState(false);
  const [cameraError, setCameraError] = useState<MediaError | null>(null);

  const handleCameraError = useCallback((error: MediaError) => {
    setCameraError(error);
  }, []);

  const handleToggleCamera = useCallback(() => {
    setCameraError(null);
    setIsCameraOpen((prev) => !prev);
  }, []);

  const handleToggleCameraExpanded = useCallback(() => {
    setIsCameraExpanded((prev) => !prev);
  }, []);

  return {
    isCameraOpen,
    isCameraExpanded,
    cameraErrorCopy: getMediaErrorCopy(cameraError),
    handleCameraError,
    handleToggleCamera,
    handleToggleCameraExpanded,
  };
}
