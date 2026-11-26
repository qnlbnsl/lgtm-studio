disable_mlock = true

storage "postgresql" {
  connection_url = "postgresql://lgtm_bao@postgres:5432/lgtm_bao?sslmode=disable"
}

seal "static" {
  current_key_id = "lgtm-1"
  current_key    = "file:///run/lgtm-bao/unseal.key"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}
