package hostsfile

import "protegewx/internal/sysexec"

// limparCacheDNS descarta o cache do resolvedor para que o hosts passe a valer
// imediatamente, sem esperar a expiracao das entradas ja resolvidas.
func limparCacheDNS() { sysexec.Rodar("ipconfig", "/flushdns") }
