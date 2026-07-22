import type { SelectHTMLAttributes } from "react";
import { fieldInput } from "./styles";

export default function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={fieldInput + " " + (props.className || "")} />;
}
