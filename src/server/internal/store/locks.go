package store

// The advisory lock keys reserved by the store.
// Parts read these constants and never invent keys in the same space.
const (
	// feedLockID serializes the final flush of feed rows.
	feedLockID int64 = 841941100001
	// migrationLockID serializes startup migrations.
	migrationLockID int64 = 841941100002
	// serverInstanceLockID is a session lock that sprint 2 startup reads.
	serverInstanceLockID int64 = 841941100003
)

// receiptLockNamespace is the first word of the two-integer receipt key space.
const receiptLockNamespace int32 = 1279743053
