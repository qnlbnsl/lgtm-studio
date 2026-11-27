package link

// OutgoingFilter is a stub of the test writer.
type OutgoingFilter struct{}

// NewOutgoingFilter is a stub of the test writer.
func NewOutgoingFilter() *OutgoingFilter { return &OutgoingFilter{} }

// AddSecrets is a stub of the test writer.
func (*OutgoingFilter) AddSecrets(values ...string) {}

// MaskText is a stub of the test writer.
func (*OutgoingFilter) MaskText(text string) string { return text }

// ContainsSecret is a stub of the test writer.
func (*OutgoingFilter) ContainsSecret(text string) bool { return false }

// TextStream is a stub of the test writer for pieces of one text.
type TextStream struct{}

// NewTextStream is a stub of the test writer.
func (*OutgoingFilter) NewTextStream() *TextStream { return &TextStream{} }

// Push is a stub of the test writer.
func (*TextStream) Push(piece []byte) string { return string(piece) }

// Flush is a stub of the test writer.
func (*TextStream) Flush() string { return "" }

// folderPathUnsafe is a stub of the test writer for the check before inspection.
func (*OutgoingFilter) folderPathUnsafe(path, home string) bool { return false }
