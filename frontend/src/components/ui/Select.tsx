import type { SelectHTMLAttributes } from "react";
import { fieldInput } from "./styles";

export default function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  // truncate: 選択中の長い文言（例: 長いアカウント名の「A → B」）を省略表示にして枠内へ収める
  return <select {...props} className={fieldInput + " truncate " + (props.className || "")} />;
}
