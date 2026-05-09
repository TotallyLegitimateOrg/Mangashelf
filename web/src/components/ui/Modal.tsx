import { useEffect, type ReactNode } from "react";
import "./Modal.css";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  actions?: ReactNode;
  width?: string;
}

export function Modal({ open, onClose, title, children, actions, width }: ModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal"
        style={width ? { maxWidth: width } : undefined}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modal__header">
          <h2 className="modal__title font-display">{title}</h2>
          <div className="modal__actions">
            {actions}
            <button className="modal__close" onClick={onClose} aria-label="Close">
              ✕
            </button>
          </div>
        </div>
        <div className="modal__body">{children}</div>
      </div>
    </div>
  );
}
