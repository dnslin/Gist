import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "../index.css";
import { DesktopShell } from "./DesktopShell";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <DesktopShell />
  </StrictMode>,
);
