// Package baseline guarda o retrato do estado das chaves e o compara com o
// estado atual. E o que sustenta o monitor diario (protegewx.exe --check).
//
// O objetivo e detectar cedo qualquer alteracao no conteudo das licencas:
// uma chave desabilitada, uma feature que deixou de ser perpetua, um dongle
// que sumiu, ou indicio de adulteracao de relogio.
package baseline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"protegewx/internal/sentinel"
)

// Retrato e o estado das chaves num instante.
type Retrato struct {
	CapturadoEm time.Time           `json:"capturadoEm"`
	Host        string              `json:"host"`
	Chaves      []sentinel.Chave    `json:"chaves"`
	Features    []sentinel.Feature  `json:"features"`
}

// Alerta e uma diferenca encontrada entre o baseline e o estado atual.
type Alerta struct {
	Gravidade string `json:"gravidade"` // critico | atencao | info
	Assunto   string `json:"assunto"`
	Detalhe   string `json:"detalhe"`
}

// Capturar le o estado atual da maquina.
func Capturar() (*Retrato, error) {
	chaves, err := sentinel.Chaves()
	if err != nil {
		return nil, err
	}
	feats, err := sentinel.Features()
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	return &Retrato{
		CapturadoEm: time.Now(),
		Host:        host,
		Chaves:      chaves,
		Features:    feats,
	}, nil
}

// Salvar grava o retrato em disco.
func Salvar(r *Retrato, caminho string) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(caminho, b, 0o644)
}

// bomUTF8 e a marca de ordem de bytes que o PowerShell adiciona ao gravar em
// UTF-8. O parser JSON da biblioteca padrao a rejeita, entao ela e descartada
// antes da leitura - do contrario um baseline gerado por script, ou apenas
// aberto e salvo no Bloco de Notas, faria o monitor falhar.
var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// Carregar le um retrato do disco.
func Carregar(caminho string) (*Retrato, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimPrefix(b, bomUTF8)

	var r Retrato
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("o arquivo de referencia %s parece corrompido: %w", caminho, err)
	}
	return &r, nil
}

// Comparar confronta o retrato de referencia com o atual e devolve os alertas.
func Comparar(ref, atual *Retrato) []Alerta {
	var as []Alerta

	refChaves := map[string]sentinel.Chave{}
	for _, c := range ref.Chaves {
		refChaves[c.HaspID] = c
	}
	atualChaves := map[string]sentinel.Chave{}
	for _, c := range atual.Chaves {
		atualChaves[c.HaspID] = c
	}

	// chaves que sumiram
	for id, c := range refChaves {
		if _, ok := atualChaves[id]; !ok {
			as = append(as, Alerta{"critico", "Dongle ausente",
				fmt.Sprintf("A chave %s (%s) estava presente no baseline e nao foi encontrada agora. "+
					"Confira se o dongle esta conectado.", id, c.Tipo)})
		}
	}
	// chaves novas
	for id, c := range atualChaves {
		if _, ok := refChaves[id]; !ok {
			as = append(as, Alerta{"info", "Dongle novo",
				fmt.Sprintf("Apareceu a chave %s (%s), que nao existia no baseline.", id, c.Tipo)})
		}
	}
	// mudancas em chave conhecida
	for id, r := range refChaves {
		a, ok := atualChaves[id]
		if !ok {
			continue
		}
		if r.KeyDisabled != a.KeyDisabled && a.KeyDisabled != "0" {
			as = append(as, Alerta{"critico", "Chave desabilitada",
				fmt.Sprintf("A chave %s passou a reportar key_disabled=%s (era %s). "+
					"Este e o sinal de desativacao da licenca.", id, a.KeyDisabled, r.KeyDisabled)})
		}
		if r.Firmware != a.Firmware {
			as = append(as, Alerta{"critico", "Firmware alterado",
				fmt.Sprintf("O firmware da chave %s mudou de %s para %s. "+
					"Isso indica que um update foi aplicado ao dongle.", id, r.Firmware, a.Firmware)})
		}
		if r.Locked != a.Locked {
			as = append(as, Alerta{"atencao", "Estado locked alterado",
				fmt.Sprintf("A chave %s mudou locked de %s para %s.", id, r.Locked, a.Locked)})
		}
		if r.Cloned != a.Cloned && a.Cloned != "0" {
			as = append(as, Alerta{"critico", "Chave marcada como clonada",
				fmt.Sprintf("A chave %s passou a reportar cloned=%s.", id, a.Cloned)})
		}
		if r.RehostType != a.RehostType {
			as = append(as, Alerta{"atencao", "Rehost alterado",
				fmt.Sprintf("A chave %s mudou rehost_type de %s para %s.", id, r.RehostType, a.RehostType)})
		}
	}

	as = append(as, compararFeatures(ref, atual)...)

	sort.SliceStable(as, func(i, j int) bool {
		return peso(as[i].Gravidade) < peso(as[j].Gravidade)
	})
	return as
}

func peso(g string) int {
	switch g {
	case "critico":
		return 0
	case "atencao":
		return 1
	default:
		return 2
	}
}

func chaveFeature(f sentinel.Feature) string { return f.HaspID + "/" + f.FeatureID }

func compararFeatures(ref, atual *Retrato) []Alerta {
	var as []Alerta

	refF := map[string]sentinel.Feature{}
	for _, f := range ref.Features {
		refF[chaveFeature(f)] = f
	}
	atualF := map[string]sentinel.Feature{}
	for _, f := range atual.Features {
		atualF[chaveFeature(f)] = f
	}

	for k, r := range refF {
		a, ok := atualF[k]
		if !ok {
			as = append(as, Alerta{"critico", "Licenca desapareceu",
				fmt.Sprintf("A feature %s (produto %q) constava no baseline e nao existe mais.",
					k, r.Produto)})
			continue
		}
		if r.Licenca != a.Licenca {
			as = append(as, Alerta{"critico", "Tipo de licenca alterado",
				fmt.Sprintf("A feature %s mudou de %q para %q. "+
					"Uma licenca perpetua que deixa de ser perpetua e o sintoma mais grave a vigiar.",
					k, r.Licenca, a.Licenca)})
		}
		if r.Desabilitada != a.Desabilitada && a.Desabilitada != "0" {
			as = append(as, Alerta{"critico", "Licenca desabilitada",
				fmt.Sprintf("A feature %s passou a reportar dis=%s.", k, a.Desabilitada)})
		}
		if r.Expirada != a.Expirada && a.Expirada != "0" {
			as = append(as, Alerta{"critico", "Licenca expirada",
				fmt.Sprintf("A feature %s passou a reportar ex=%s.", k, a.Expirada)})
		}
		if r.Inutilizavel != a.Inutilizavel && a.Inutilizavel != "0" {
			as = append(as, Alerta{"critico", "Licenca inutilizavel",
				fmt.Sprintf("A feature %s passou a reportar unusable=%s.", k, a.Inutilizavel)})
		}
		if a.RelogioMex != "" && r.RelogioMex != a.RelogioMex {
			as = append(as, Alerta{"atencao", "Indicio de relogio adulterado",
				fmt.Sprintf("A feature %s reporta time_tampered=%q.", k, a.RelogioMex)})
		}
	}
	for k, a := range atualF {
		if _, ok := refF[k]; !ok {
			as = append(as, Alerta{"info", "Licenca nova",
				fmt.Sprintf("Apareceu a feature %s (produto %q).", k, a.Produto)})
		}
	}
	return as
}
