import { useEffect } from "react";
import { Router, useLocation } from "wouter";
import { useTranslation } from "react-i18next";
import { LoginPage, RegisterPage, NetworkErrorPage } from "@/components/auth";
import { ReaderWorkspace } from "@/components/reader-workspace";
import { TooltipProvider } from "@/components/ui/tooltip";
import { UpdateNotice } from "@/components/update-notice";
import { useAuth } from "@/hooks/useAuth";

function LoadingScreen() {
  const { t } = useTranslation();
  return (
    <div className="flex h-full w-full items-center justify-center overflow-x-clip bg-background">
      <div className="flex flex-col items-center gap-4">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        <p className="text-sm text-muted-foreground">{t("entry.loading")}</p>
      </div>
    </div>
  );
}

function AppContent() {
  const [location, navigate] = useLocation();
  const {
    isLoading,
    isAuthenticated,
    needsRegistration,
    needsLogin,
    isNetworkError,
    error,
    shouldRedirectToRoot,
    login,
    register,
    retry,
    clearError,
    consumeRootRedirect,
  } = useAuth();

  useEffect(() => {
    if (!shouldRedirectToRoot) {
      return;
    }
    if (location !== "/") {
      navigate("/", { replace: true });
    }
    consumeRootRedirect();
  }, [shouldRedirectToRoot, location, navigate, consumeRootRedirect]);

  if (isLoading) {
    return <LoadingScreen />;
  }

  if (isNetworkError) {
    return <NetworkErrorPage onRetry={retry} />;
  }

  if (needsRegistration) {
    return (
      <RegisterPage
        onRegister={register}
        error={error}
        onClearError={clearError}
      />
    );
  }

  if (needsLogin) {
    return (
      <LoginPage onLogin={login} error={error} onClearError={clearError} />
    );
  }

  if (isAuthenticated) {
    return <ReaderWorkspace />;
  }

  return <LoadingScreen />;
}

function App() {
  return (
    <div className="app-shell">
      <TooltipProvider delayDuration={300}>
        <Router>
          <AppContent />
          <UpdateNotice />
        </Router>
      </TooltipProvider>
    </div>
  );
}

export default App;
