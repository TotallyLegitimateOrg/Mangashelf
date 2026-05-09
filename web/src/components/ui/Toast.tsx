/* ============================================================
   Toast notification system
   ============================================================ */
import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useRef,
  type ReactNode,
} from "react";
import "./Toast.css";

type ToastType = "success" | "error" | "info";

interface Toast {
  id: number;
  message: string;
  type: ToastType;
  leaving?: boolean;
}

interface ToastContextValue {
  toast: (message: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const autoDismissTimers = useRef(new Map<number, number>());
  const removeTimers = useRef(new Map<number, number>());

  const clearToastTimers = useCallback((id: number) => {
    const autoDismissTimer = autoDismissTimers.current.get(id);
    if (autoDismissTimer !== undefined) {
      window.clearTimeout(autoDismissTimer);
      autoDismissTimers.current.delete(id);
    }

    const removeTimer = removeTimers.current.get(id);
    if (removeTimer !== undefined) {
      window.clearTimeout(removeTimer);
      removeTimers.current.delete(id);
    }
  }, []);

  const dismissToast = useCallback((id: number) => {
    clearToastTimers(id);
    setToasts((prev) =>
      prev.map((t) => (t.id === id && !t.leaving ? { ...t, leaving: true } : t))
    );
    const removeTimer = window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
      removeTimers.current.delete(id);
    }, 300);
    removeTimers.current.set(id, removeTimer);
  }, [clearToastTimers]);

  const toast = useCallback((message: string, type: ToastType = "info") => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type }]);
    const autoDismissTimer = window.setTimeout(() => {
      dismissToast(id);
    }, 4000);
    autoDismissTimers.current.set(id, autoDismissTimer);
  }, [dismissToast]);

  useEffect(() => () => {
    for (const timer of autoDismissTimers.current.values()) {
      window.clearTimeout(timer);
    }
    for (const timer of removeTimers.current.values()) {
      window.clearTimeout(timer);
    }
    autoDismissTimers.current.clear();
    removeTimers.current.clear();
  }, []);

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="toast-container" role="alert" aria-live="polite">
        {toasts.map((t) => (
          <button
            key={t.id}
            type="button"
            className={`toast toast--${t.type} ${t.leaving ? "toast--leaving" : ""}`}
            onClick={() => dismissToast(t.id)}
            aria-label={`Dismiss notification: ${t.message}`}
          >
            <span className="toast__icon">
              {t.type === "success" ? "✓" : t.type === "error" ? "✕" : "ℹ"}
            </span>
            <span className="toast__message">{t.message}</span>
          </button>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}
