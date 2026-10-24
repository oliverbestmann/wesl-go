import { _linkSync, LinkedWesl } from "wesl";

// wesl's link() is async but does all work synchronously. Calling the sync
// core lets Go read the result without driving the JS event loop, which
// would deadlock under GOOS=js when called from a js.FuncOf callback.
export function link(opts) {
  return new LinkedWesl(_linkSync(opts)).dest;
}
