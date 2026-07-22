import type { InputHTMLAttributes } from "react";
import { fieldInput } from "./styles";

export default function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={fieldInput + " " + (props.className || "")} />;
}
