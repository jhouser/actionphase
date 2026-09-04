package observability

// RFC 7807 problem details primitives.
//
// These live in observability rather than core because core imports this
// package, and the panic recovery middleware here has to emit a problem
// document without reaching back into core. core.ErrResponse is the richer
// renderer built on the same vocabulary.

// ProblemContentType is the media type RFC 7807 mandates for problem details.
// Every error emitter uses this one value so clients can content-negotiate on
// a single string.
const ProblemContentType = "application/problem+json; charset=utf-8"

// correlationURNPrefix namespaces correlation IDs as a URN, giving `instance`
// the URI reference RFC 7807 asks for rather than a bare opaque ID.
const correlationURNPrefix = "urn:actionphase:correlation:"

// problemJSON is the minimal RFC 7807 document, used where the full
// core.ErrResponse renderer is not reachable.
type problemJSON struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// CorrelationInstance renders a correlation ID as the URI reference RFC 7807
// expects in `instance`. Returns empty for an empty ID so the field is omitted
// rather than emitted as a bare prefix.
func CorrelationInstance(correlationID string) string {
	if correlationID == "" {
		return ""
	}
	return correlationURNPrefix + correlationID
}
