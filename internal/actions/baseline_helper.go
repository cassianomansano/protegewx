package actions

import (
	"path/filepath"

	"protegewx/internal/baseline"
)

func caminhoBaseline(c *Ctx) string {
	return filepath.Join(c.Base, "baseline", "chaves-baseline.json")
}

func salvarBaseline(c *Ctx) error {
	r, err := baseline.Capturar()
	if err != nil {
		return err
	}
	return baseline.Salvar(r, caminhoBaseline(c))
}

// Conferir compara o estado atual com o baseline gravado.
func Conferir(c *Ctx) ([]baseline.Alerta, error) {
	ref, err := baseline.Carregar(caminhoBaseline(c))
	if err != nil {
		return nil, err
	}
	atual, err := baseline.Capturar()
	if err != nil {
		return nil, err
	}
	return baseline.Comparar(ref, atual), nil
}
