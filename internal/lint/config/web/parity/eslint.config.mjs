// The parity set: sonarjs recommended and the rules of the public rule map
// at error, so a repository green here is expected green under Sonar way for
// every mapped key, and nothing a Sonar analysis would not raise.
import { webFrame } from './golaw-web.mjs'

const frame = await webFrame()

export default [...frame.base, ...frame.parity, ...frame.closing]
