interface WordmarkProps {
  // ワードマーク全体（"duo" 部分）の色・サイズなど
  className?: string;
  // 中黒 "·" と "pocketbook" 部分の色（既定はブルーアクセント）
  subClassName?: string;
}

// ブランドロゴ。ログイン画面とヘッダーで共通利用し、表記・書体を揃える。
// ハイフンではなく中黒（·）で 2 語をつなぎ、丸みのある書体（font-brand）で表示する。
export default function Wordmark({
  className = "",
  subClassName = "text-blue-600 dark:text-blue-300",
}: WordmarkProps) {
  return (
    <span className={`font-brand font-bold lowercase tracking-tight ${className}`}>
      duo
      <span className={`font-light ${subClassName}`}>
        <span className="mx-[0.06em]">·</span>pocketbook
      </span>
    </span>
  );
}
