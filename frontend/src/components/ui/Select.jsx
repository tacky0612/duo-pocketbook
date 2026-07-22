import { fieldInput } from "./styles.js";

export default function Select(props) {
  return <select {...props} className={fieldInput + " " + (props.className || "")} />;
}
