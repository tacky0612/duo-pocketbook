import { useEffect, type ReactNode } from "react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children?: ReactNode;
  // ダイアログ下部に並べるボタン類。
  footer?: ReactNode;
}

// 画面中央（モバイルでは下端）に表示するモーダルダイアログ。
// 背景クリック・Escape キーで閉じる。中身が長い場合は本文だけをスクロールさせる。
export default function Modal({ open, onClose, title, children, footer }: ModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-40 flex items-end justify-center sm:items-center sm:p-4">
      <div aria-hidden className="absolute inset-0 bg-slate-950/50 backdrop-blur-sm" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        className="relative flex max-h-[90vh] w-full flex-col rounded-t-2xl bg-white shadow-xl motion-safe:animate-[panel-in_0.3s_cubic-bezier(0.22,1,0.36,1)] sm:max-w-lg sm:rounded-2xl dark:bg-slate-900"
      >
        <div className="border-b border-slate-100 px-5 py-4 dark:border-slate-800">
          <h2 className="text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100">{title}</h2>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex flex-col-reverse gap-2 border-t border-slate-100 px-5 py-4 pb-[max(1rem,env(safe-area-inset-bottom))] sm:flex-row sm:justify-end dark:border-slate-800">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
