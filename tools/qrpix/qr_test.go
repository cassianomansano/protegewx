package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------- decodificação
//
// O decodificador abaixo existe só para o teste. Ele desfaz cada etapa do
// gerador — máscara, ziguezague, intercalação — e confere se o texto original
// reaparece. Um erro em qualquer uma dessas etapas quebra o round-trip.

// lerFormato extrai nível e máscara da primeira cópia da informação de formato.
func lerFormato(m *Matriz) (Nivel, int, bool) {
	bits := 0
	for i := 0; i < 15; i++ {
		var v int8
		switch {
		case i < 6:
			v = m.get(8, i)
		case i == 6:
			v = m.get(8, 7)
		case i == 7:
			v = m.get(8, 8)
		case i == 8:
			v = m.get(7, 8)
		default:
			v = m.get(14-i, 8)
		}
		bits |= int(v) << i
	}
	bits ^= 0x5412

	// confere o BCH: num código válido o resto da divisão é zero
	if resto(bits, 0x537, 14, 10) != 0 {
		return 0, 0, false
	}

	dados := bits >> 10
	nivel := NivelM
	if dados>>3 == 0b01 {
		nivel = NivelL
	}
	return nivel, dados & 0b111, true
}

// extrairBits percorre a matriz na mesma ordem do preenchimento e devolve os
// codewords intercalados, já sem a máscara.
func extrairBits(m *Matriz, mascara int) []byte {
	limpa := novaMatriz(m.tam)
	copy(limpa.celula, m.celula)
	copy(limpa.funcao, m.funcao)
	limpa.aplicarMascara(mascara) // aplicar de novo desfaz, por ser XOR

	var saida []byte
	bit := 0
	var atual byte
	subindo := true

	for direita := m.tam - 1; direita >= 1; direita -= 2 {
		if direita == 6 {
			direita = 5
		}
		for i := 0; i < m.tam; i++ {
			l := i
			if subindo {
				l = m.tam - 1 - i
			}
			for _, c := range []int{direita, direita - 1} {
				if limpa.ehFuncao(l, c) {
					continue
				}
				if limpa.get(l, c) == 1 {
					atual |= 1 << (7 - bit%8)
				}
				bit++
				if bit%8 == 0 {
					saida = append(saida, atual)
					atual = 0
				}
			}
		}
		subindo = !subindo
	}
	return saida
}

// desintercalar reconstrói os blocos de dados a partir do fluxo intercalado.
func desintercalar(fluxo []byte, versao int, nivel Nivel) ([]byte, [][]byte, [][]byte) {
	b := tabelaBlocos[nivel][versao]

	tamanhos := make([]int, 0, b.totalBlocos())
	for i := 0; i < b.g1Blocos; i++ {
		tamanhos = append(tamanhos, b.g1Dados)
	}
	for i := 0; i < b.g2Blocos; i++ {
		tamanhos = append(tamanhos, b.g2Dados)
	}

	blocos := make([][]byte, len(tamanhos))
	maior := 0
	for i, t := range tamanhos {
		blocos[i] = make([]byte, 0, t)
		if t > maior {
			maior = t
		}
	}

	pos := 0
	for i := 0; i < maior; i++ {
		for j, t := range tamanhos {
			if i < t {
				blocos[j] = append(blocos[j], fluxo[pos])
				pos++
			}
		}
	}

	ec := make([][]byte, len(tamanhos))
	for i := range ec {
		ec[i] = make([]byte, 0, b.ecPorBloco)
	}
	for i := 0; i < b.ecPorBloco; i++ {
		for j := range ec {
			ec[j] = append(ec[j], fluxo[pos])
			pos++
		}
	}

	var dados []byte
	for _, bl := range blocos {
		dados = append(dados, bl...)
	}
	return dados, blocos, ec
}

// lerTexto interpreta o cabeçalho de modo byte e devolve o conteúdo.
func lerTexto(dados []byte, versao int) string {
	if len(dados) < 2 || dados[0]>>4 != 0b0100 {
		return ""
	}
	bitsContador := 8
	if versao >= 10 {
		bitsContador = 16
	}

	ler := func(inicio, quantidade int) int {
		v := 0
		for i := 0; i < quantidade; i++ {
			p := inicio + i
			bit := (dados[p/8] >> (7 - p%8)) & 1
			v = v<<1 | int(bit)
		}
		return v
	}

	n := ler(4, bitsContador)
	inicio := 4 + bitsContador
	if n <= 0 || (inicio+n*8) > len(dados)*8 {
		return ""
	}

	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = byte(ler(inicio+i*8, 8))
	}
	return string(out)
}

// decodificar refaz o caminho inteiro e devolve o texto lido.
func decodificar(m *Matriz, versao int) (string, bool) {
	nivel, mascara, ok := lerFormato(m)
	if !ok {
		return "", false
	}
	fluxo := extrairBits(m, mascara)
	dados, blocos, ec := desintercalar(fluxo, versao, nivel)

	// a correção de erro precisa bater com o que seria calculado dos dados
	b := tabelaBlocos[nivel][versao]
	for i, bl := range blocos {
		esperado := correcaoErro(bl, b.ecPorBloco)
		for j := range esperado {
			if esperado[j] != ec[i][j] {
				return "", false
			}
		}
	}
	return lerTexto(dados, versao), true
}

// ---------------------------------------------------------------- testes

func TestRoundTrip(t *testing.T) {
	casos := []string{
		"HELLO",
		"https://github.com/cassianomansano/protegewx",
		MontarBRCode("c6@exemplo.com.br", "PROTEGEWX", "Dourados"),
		MontarBRCode("11111111-2222-3333-4444-555555555555", "FULANO DE TAL", "SAO PAULO"),
		strings.Repeat("A", 100),
	}

	for _, texto := range casos {
		m, versao, err := Gerar(texto, NivelM)
		if err != nil {
			t.Fatalf("Gerar(%.30q): %v", texto, err)
		}
		lido, ok := decodificar(m, versao)
		if !ok {
			t.Fatalf("decodificação falhou para %.40q (versão %d)", texto, versao)
		}
		if lido != texto {
			t.Errorf("round-trip divergiu\n  enviado: %q\n  lido   : %q", texto, lido)
		}
	}
}

// TestEstrutura confere os elementos fixos que todo QR Code precisa ter.
func TestEstrutura(t *testing.T) {
	m, versao, err := Gerar("teste de estrutura", NivelM)
	if err != nil {
		t.Fatal(err)
	}

	// os três finder patterns têm centro escuro e anel claro em volta
	for _, p := range [][2]int{{0, 0}, {0, m.tam - 7}, {m.tam - 7, 0}} {
		if m.get(p[0]+3, p[1]+3) != 1 {
			t.Errorf("centro do finder em (%d,%d) deveria ser escuro", p[0], p[1])
		}
		if m.get(p[0]+1, p[1]+1) != 0 {
			t.Errorf("anel do finder em (%d,%d) deveria ser claro", p[0], p[1])
		}
	}

	// timing pattern alterna a partir da coluna 8
	for i := 8; i < m.tam-8; i++ {
		esperado := int8(0)
		if i%2 == 0 {
			esperado = 1
		}
		if m.get(6, i) != esperado {
			t.Errorf("timing horizontal errado na coluna %d", i)
		}
		if m.get(i, 6) != esperado {
			t.Errorf("timing vertical errado na linha %d", i)
		}
	}

	// módulo escuro obrigatório
	if m.get(4*versao+9, 8) != 1 {
		t.Error("o módulo escuro fixo não está presente")
	}

	// nenhum módulo pode ficar indefinido
	for i, v := range m.celula {
		if v < 0 {
			t.Fatalf("módulo (%d,%d) ficou sem definição", i/m.tam, i%m.tam)
		}
	}
}

func TestCRC16(t *testing.T) {
	// valor conhecido do padrão EMV: "123456789" resulta em 0x29B1
	if got := crc16("123456789"); got != "29B1" {
		t.Errorf("crc16 = %s, esperado 29B1", got)
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"José da Silvá Ção": "JOSE DA SILVA CAO",
		"São Paulo":         "SAO PAULO",
		"Dourados":          "DOURADOS",
	}
	for entrada, esperado := range casos {
		if got := normalizar(entrada, 25); got != esperado {
			t.Errorf("normalizar(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
}
