package messages

import (
	"time"

	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time verification that MessageService implements MessageServiceInterface
// TODO: Uncomment after all methods are migrated
// var _ core.MessageServiceInterface = (*MessageService)(nil)

// MessageService handles message and comment operations for the Common Room and private messaging.
// All messages must be sent as characters and are associated with a game.
type MessageService struct {
	DB      *pgxpool.Pool
	Logger  *observability.Logger
	Metrics *observability.OTELMetrics
}

// Helper function to convert *int32 to pgtype.Int4
func int32ToPgInt4(val *int32) pgtype.Int4 {
	if val == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *val, Valid: true}
}

// Helper function to convert int32 to pgtype.Int4
func int32ValueToPgInt4(val int32) pgtype.Int4 {
	return pgtype.Int4{Int32: val, Valid: true}
}

// Helper function to convert pgtype.Int4 to *int32
func pgInt4ToInt32Ptr(val pgtype.Int4) *int32 {
	if !val.Valid {
		return nil
	}
	return &val.Int32
}

// Helper function to convert pgtype.Timestamp to time.Time
func pgTimestampToTime(val pgtype.Timestamp) time.Time {
	return val.Time
}

// Helper function to convert pgtype.Timestamp to *time.Time
func pgTimestampToTimePtr(val pgtype.Timestamp) *time.Time {
	if !val.Valid {
		return nil
	}
	return &val.Time
}

// Helper function to convert pgtype.Timestamptz to *time.Time
func pgTimestamptzToTimePtr(val pgtype.Timestamptz) *time.Time {
	if !val.Valid {
		return nil
	}
	return &val.Time
}

// Helper function to convert pgtype.Text to *string
func pgTextToStringPtr(val pgtype.Text) *string {
	if !val.Valid {
		return nil
	}
	return &val.String
}

// Helper function to convert pgtype.Bool to *bool
func pgBoolToBoolPtr(val pgtype.Bool) *bool {
	if !val.Valid {
		return nil
	}
	return &val.Bool
}

// Helper function to convert NullMessageType to *string
func nullMessageTypeToStringPtr(val models.NullMessageType) *string {
	if !val.Valid {
		return nil
	}
	str := string(val.MessageType)
	return &str
}
