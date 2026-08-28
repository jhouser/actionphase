package messages

import (
	"strings"

	"actionphase/pkg/validation"

	"github.com/danielgtaylor/huma/v2"
)

// Request bodies
//
// Length rules live in struct tags where the chi handler enforced them, so huma
// checks them before the handler runs and publishes them in the spec. Resolve
// handles only what the schema cannot: trimming, since huma's minLength counts
// raw characters and a body of "   " would otherwise satisfy minLength:"1" and
// store a blank message.
//
// Note the deliberate asymmetry in maxLength: the chi handlers called
// validation.ValidatePost on update-post and both draft writes, but NOT on
// create-post or create-comment. Adding the cap to create would reject payloads
// that work today, so the tags mirror the handlers rather than tidying them up.
// Whether create should be capped too is a real question, but not one a
// behaviour-preserving port gets to answer.

// requiredContent trims *s in place and reports it missing if nothing is left.
func requiredContent(s *string) []error {
	*s = strings.TrimSpace(*s)
	if *s != "" {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  "content is required",
		Location: "body.content",
	}}
}

type CreatePostRequest struct {
	PhaseID     *int32 `json:"phase_id,omitempty" required:"false" doc:"Phase to attach the post to"`
	CharacterID int32  `json:"character_id" minimum:"1" doc:"Character to attribute the post to"`
	Content     string `json:"content" minLength:"1" doc:"Post body, as markdown"`
}

func (r *CreatePostRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

type CreateCommentRequest struct {
	PhaseID     *int32 `json:"phase_id,omitempty" required:"false" doc:"Phase to attach the comment to"`
	CharacterID int32  `json:"character_id" minimum:"1" doc:"Character to attribute the comment to"`
	Content     string `json:"content" minLength:"1" doc:"Comment body, as markdown"`
	// The path's {postId} is the immediate parent, which for a nested reply is
	// another comment rather than the post. Read tracking keys off the top-level
	// post, so clients must send it explicitly once they reply below depth 0.
	// Omitting it falls back to {postId}, which is correct only for a direct
	// reply to a post.
	RootPostID *int32 `json:"root_post_id,omitempty" required:"false" doc:"Top-level post of this thread. Required for replies nested below a post; defaults to the path's postId."`
}

func (r *CreateCommentRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

type UpdateCommentRequest struct {
	Content string `json:"content" minLength:"1" doc:"Replacement comment body"`
	// Lets an author re-attribute a comment to another character they control.
	CharacterID *int32 `json:"character_id,omitempty" required:"false" doc:"Character to re-attribute the comment to; must be one the caller controls"`
}

func (r *UpdateCommentRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

type UpdatePostRequest struct {
	Content string `json:"content" minLength:"1" maxLength:"50000" doc:"Replacement post body, as markdown"`
}

func (r *UpdatePostRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

type CreateDraftPostRequest struct {
	CharacterID int32  `json:"character_id" minimum:"1" doc:"Character to attribute the draft to"`
	Content     string `json:"content" minLength:"1" maxLength:"50000" doc:"Draft body, as markdown"`
}

func (r *CreateDraftPostRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

type UpdateDraftPostRequest struct {
	Content string `json:"content" minLength:"1" maxLength:"50000" doc:"Replacement draft body, as markdown"`
}

func (r *UpdateDraftPostRequest) Resolve(huma.Context) []error {
	return requiredContent(&r.Content)
}

// MarkPostReadRequest records how far the caller has read in a thread.
//
// The whole body is optional -- the chi handler tolerated an empty body via
// io.EOF -- so marking a post read without naming a comment is a valid request.
type MarkPostReadRequest struct {
	LastReadCommentID *int32 `json:"last_read_comment_id,omitempty" required:"false" doc:"Newest comment read; omit to mark only the post itself"`
}

// ToggleCommentReadRequest sets or clears the manual read flag on one comment.
type ToggleCommentReadRequest struct {
	Read bool `json:"read" required:"false" doc:"true marks the comment read, false marks it unread"`
}

// maxPostContentLength is validation.MaxPostLength, restated as the maxLength
// tag value above. Kept as a compile-time check so the two cannot drift.
const maxPostContentLength = 50000

var _ = [1]struct{}{}[maxPostContentLength-validation.MaxPostLength]
