-- The public pin of the machine-link signing key. No private key or credential is stored here.
CREATE TABLE server_link_identity (
    singleton boolean PRIMARY KEY CHECK (singleton),
    fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$')
);
