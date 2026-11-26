package vault

// Adopted reports that this load wrote the first pin for a key it did not create.
// The key was already in OpenBao, or a competing create stored it first.
// A load under an established pin and a load that created its own key report false.
func (id LinkIdentity) Adopted() bool { return id.adopted }
