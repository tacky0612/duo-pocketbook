import { normalizeDigits } from "../../lib/format.js";
import { fieldInput } from "./styles.js";

// 数値入力。全角数字も受け付け、半角数字のみへ正規化して onChange(値の文字列) を呼ぶ。
// type="text" + inputMode="numeric" とすることでブラウザに全角入力を弾かせない。
export default function NumberInput({ value, onChange, className = "", ...props }) {
  return (
    <input
      {...props}
      type="text"
      inputMode="numeric"
      pattern="[0-9]*"
      value={value}
      onChange={(e) => onChange(normalizeDigits(e.target.value))}
      className={fieldInput + " " + className}
    />
  );
}
