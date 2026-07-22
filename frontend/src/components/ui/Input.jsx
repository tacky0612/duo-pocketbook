import { fieldInput } from "./styles.js";

export default function Input(props) {
  return <input {...props} className={fieldInput + " " + (props.className || "")} />;
}
