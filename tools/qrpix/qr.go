// Gerador de QR Code, escrito do zero sobre a biblioteca padrão.
//
// Implementa o subconjunto da ISO/IEC 18004 necessário para um BR Code do PIX:
// modo byte, versões 1 a 10, correção de erro nível L ou M.
//
// A ordem das etapas segue a norma:
//
//	texto -> codewords de dados -> Reed-Solomon -> intercalação dos blocos
//	      -> desenho dos padrões de função -> preenchimento em ziguezague
//	      -> escolha da máscara por penalidade -> informação de formato
package main

import (
	"errors"
	"strings"
)

// Nivel de correção de erro.
type Nivel int

const (
	NivelL Nivel = iota // recupera ~7%
	NivelM              // recupera ~15%
)

// bitsNivel são os dois bits que representam o nível na informação de formato.
// A ordem não é sequencial: é a definida pela norma.
func (n Nivel) bits() int {
	if n == NivelL {
		return 0b01
	}
	return 0b00
}

// bloco descreve como os codewords de uma versão/nível são divididos.
type bloco struct {
	ecPorBloco int // codewords de correção em cada bloco
	g1Blocos   int // quantidade de blocos do grupo 1
	g1Dados    int // codewords de dados em cada bloco do grupo 1
	g2Blocos   int // quantidade de blocos do grupo 2 (0 se não houver)
	g2Dados    int
}

// tabelaBlocos[nivel][versao] — versão 0 não existe, fica zerada.
// Valores da tabela 9 da ISO/IEC 18004.
var tabelaBlocos = map[Nivel][11]bloco{
	NivelL: {
		{}, // versão 0 (inexistente)
		{7, 1, 19, 0, 0},
		{10, 1, 34, 0, 0},
		{15, 1, 55, 0, 0},
		{20, 1, 80, 0, 0},
		{26, 1, 108, 0, 0},
		{18, 2, 68, 0, 0},
		{20, 2, 78, 0, 0},
		{24, 2, 97, 0, 0},
		{30, 2, 116, 0, 0},
		{18, 2, 68, 2, 69},
	},
	NivelM: {
		{},
		{10, 1, 16, 0, 0},
		{16, 1, 28, 0, 0},
		{26, 1, 44, 0, 0},
		{18, 2, 32, 0, 0},
		{24, 2, 43, 0, 0},
		{16, 4, 27, 0, 0},
		{18, 4, 31, 0, 0},
		{22, 2, 38, 2, 39},
		{22, 3, 36, 2, 37},
		{26, 4, 43, 1, 44},
	},
}

// posAlinhamento traz as coordenadas dos padrões de alinhamento por versão.
var posAlinhamento = [11][]int{
	{}, {}, // versões 0 e 1 não têm
	{6, 18}, {6, 22}, {6, 26}, {6, 30}, {6, 34},
	{6, 22, 38}, {6, 24, 42}, {6, 26, 46}, {6, 28, 50},
}

func (b bloco) totalDados() int { return b.g1Blocos*b.g1Dados + b.g2Blocos*b.g2Dados }
func (b bloco) totalBlocos() int { return b.g1Blocos + b.g2Blocos }

// ---------------------------------------------------------------- GF(256)

// O corpo finito usado pelo Reed-Solomon do QR Code tem polinômio primitivo
// 0x11D. As tabelas de exponencial e logaritmo transformam multiplicação em
// soma de expoentes, que é o que torna o cálculo viável.
var (
	gfExp [512]byte
	gfLog [256]byte
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	// duplicar a tabela evita ter que fazer módulo 255 a cada multiplicação
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// polinomioGerador devolve o polinômio gerador de grau n usado no Reed-Solomon.
func polinomioGerador(n int) []byte {
	g := []byte{1}
	for i := 0; i < n; i++ {
		// multiplica g por (x - alfa^i)
		novo := make([]byte, len(g)+1)
		for j, c := range g {
			novo[j] ^= gfMul(c, gfExp[i])
			novo[j+1] ^= c
		}
		g = novo
	}
	return g
}

// correcaoErro calcula os codewords de correção de um bloco de dados.
func correcaoErro(dados []byte, n int) []byte {
	g := polinomioGerador(n)
	resto := make([]byte, len(dados)+n)
	copy(resto, dados)

	for i := 0; i < len(dados); i++ {
		coef := resto[i]
		if coef == 0 {
			continue
		}
		for j, gc := range g {
			resto[i+j] ^= gfMul(gc, coef)
		}
	}
	return resto[len(dados):]
}

// ---------------------------------------------------------------- bits

type fluxoBits struct {
	bytes []byte
	n     int // quantidade de bits já escritos
}

func (f *fluxoBits) push(valor, quantidade int) {
	for i := quantidade - 1; i >= 0; i-- {
		if f.n%8 == 0 {
			f.bytes = append(f.bytes, 0)
		}
		if valor&(1<<i) != 0 {
			f.bytes[f.n/8] |= 1 << (7 - f.n%8)
		}
		f.n++
	}
}

// ---------------------------------------------------------------- codificação

// escolherVersao acha a menor versão que comporta o texto no nível pedido.
func escolherVersao(tamanho int, nivel Nivel) (int, error) {
	for v := 1; v <= 10; v++ {
		b := tabelaBlocos[nivel][v]
		// 4 bits de modo + 8 bits de contador (versões 1 a 9) ou 16 (10+)
		bitsCabecalho := 4 + 8
		if v >= 10 {
			bitsCabecalho = 4 + 16
		}
		if bitsCabecalho+tamanho*8 <= b.totalDados()*8 {
			return v, nil
		}
	}
	return 0, errors.New("texto longo demais para as versões suportadas (1 a 10)")
}

// codificarDados produz os codewords finais, já com correção de erro e
// blocos intercalados na ordem que a norma exige.
func codificarDados(texto string, versao int, nivel Nivel) []byte {
	b := tabelaBlocos[nivel][versao]

	f := &fluxoBits{}
	f.push(0b0100, 4) // modo byte
	if versao >= 10 {
		f.push(len(texto), 16)
	} else {
		f.push(len(texto), 8)
	}
	for _, c := range []byte(texto) {
		f.push(int(c), 8)
	}

	// terminador de até 4 bits, sem ultrapassar a capacidade
	capacidadeBits := b.totalDados() * 8
	if sobra := capacidadeBits - f.n; sobra > 0 {
		if sobra > 4 {
			sobra = 4
		}
		f.push(0, sobra)
	}
	// completar o byte corrente
	if f.n%8 != 0 {
		f.push(0, 8-f.n%8)
	}
	// preencher o resto alternando os dois bytes de enchimento da norma
	preenchimento := []byte{0xEC, 0x11}
	for i := 0; len(f.bytes) < b.totalDados(); i++ {
		f.bytes = append(f.bytes, preenchimento[i%2])
	}

	// dividir em blocos
	var blocosDados [][]byte
	var blocosEC [][]byte
	pos := 0
	adicionar := func(quantidade, tamanho int) {
		for i := 0; i < quantidade; i++ {
			d := f.bytes[pos : pos+tamanho]
			pos += tamanho
			blocosDados = append(blocosDados, d)
			blocosEC = append(blocosEC, correcaoErro(d, b.ecPorBloco))
		}
	}
	adicionar(b.g1Blocos, b.g1Dados)
	adicionar(b.g2Blocos, b.g2Dados)

	// intercalar: um codeword de cada bloco por vez
	var saida []byte
	maiorDados := b.g1Dados
	if b.g2Dados > maiorDados {
		maiorDados = b.g2Dados
	}
	for i := 0; i < maiorDados; i++ {
		for _, d := range blocosDados {
			if i < len(d) {
				saida = append(saida, d[i])
			}
		}
	}
	for i := 0; i < b.ecPorBloco; i++ {
		for _, e := range blocosEC {
			saida = append(saida, e[i])
		}
	}
	return saida
}

// ---------------------------------------------------------------- matriz

// Matriz é o desenho do QR Code. -1 significa módulo ainda não definido.
type Matriz struct {
	tam    int
	celula []int8
	funcao []bool // marca os módulos de função, que não recebem dados
}

func novaMatriz(tam int) *Matriz {
	m := &Matriz{tam: tam, celula: make([]int8, tam*tam), funcao: make([]bool, tam*tam)}
	for i := range m.celula {
		m.celula[i] = -1
	}
	return m
}

func (m *Matriz) get(l, c int) int8     { return m.celula[l*m.tam+c] }
func (m *Matriz) set(l, c int, v int8)  { m.celula[l*m.tam+c] = v }
func (m *Matriz) ehFuncao(l, c int) bool { return m.funcao[l*m.tam+c] }

func (m *Matriz) setFuncao(l, c int, v int8) {
	if l < 0 || c < 0 || l >= m.tam || c >= m.tam {
		return
	}
	m.set(l, c, v)
	m.funcao[l*m.tam+c] = true
}

// desenharPadroes coloca finder, separadores, timing, alinhamento e o módulo
// escuro fixo — tudo que não carrega dados.
func (m *Matriz) desenharPadroes(versao int) {
	// três finder patterns, com os separadores brancos em volta
	for _, p := range [][2]int{{0, 0}, {0, m.tam - 7}, {m.tam - 7, 0}} {
		for dl := -1; dl <= 7; dl++ {
			for dc := -1; dc <= 7; dc++ {
				l, c := p[0]+dl, p[1]+dc
				if l < 0 || c < 0 || l >= m.tam || c >= m.tam {
					continue
				}
				borda := dl == 0 || dl == 6 || dc == 0 || dc == 6
				centro := dl >= 2 && dl <= 4 && dc >= 2 && dc <= 4
				dentro := dl >= 0 && dl <= 6 && dc >= 0 && dc <= 6
				var v int8
				if dentro && (borda || centro) {
					v = 1
				}
				m.setFuncao(l, c, v)
			}
		}
	}

	// timing patterns: linha e coluna 6, alternando
	for i := 8; i < m.tam-8; i++ {
		var v int8
		if i%2 == 0 {
			v = 1
		}
		m.setFuncao(6, i, v)
		m.setFuncao(i, 6, v)
	}

	// padrões de alinhamento, exceto onde colidiriam com os finder
	pos := posAlinhamento[versao]
	for _, l := range pos {
		for _, c := range pos {
			if (l == 6 && c == 6) || (l == 6 && c == m.tam-7) || (l == m.tam-7 && c == 6) {
				continue
			}
			for dl := -2; dl <= 2; dl++ {
				for dc := -2; dc <= 2; dc++ {
					var v int8
					if dl == -2 || dl == 2 || dc == -2 || dc == 2 || (dl == 0 && dc == 0) {
						v = 1
					}
					m.setFuncao(l+dl, c+dc, v)
				}
			}
		}
	}

	// Reservar as áreas da informação de formato.
	//
	// Dois cuidados aqui, e ambos custaram teste vermelho: a linha 8 e a coluna
	// 8 cruzam o timing pattern na posição 6, que não pode ser sobrescrito; e a
	// cópia inferior ocupa 7 módulos, não 8 — o oitavo é o módulo escuro fixo.
	for i := 0; i < 9; i++ {
		if i != 6 {
			m.setFuncao(8, i, 0)
			m.setFuncao(i, 8, 0)
		}
	}
	for i := 0; i < 8; i++ {
		m.setFuncao(8, m.tam-1-i, 0)
	}
	for i := 0; i < 7; i++ {
		m.setFuncao(m.tam-1-i, 8, 0)
	}

	// módulo escuro obrigatório, depois da reserva para não ser apagado
	m.setFuncao(4*versao+9, 8, 1)

	// informação de versão, presente a partir da versão 7
	if versao >= 7 {
		bits := infoVersao(versao)
		for i := 0; i < 18; i++ {
			v := int8((bits >> i) & 1)
			l, c := i/3, i%3
			m.setFuncao(m.tam-11+c, l, v)
			m.setFuncao(l, m.tam-11+c, v)
		}
	}
}

// resto divide o polinômio em `valor` pelo gerador `gerador` em GF(2) e devolve
// o resto. `grauMax` é o grau do dividendo e `grauGer` o do gerador.
//
// É a divisão longa binária: enquanto houver bit ligado acima do grau do
// gerador, alinha-se o gerador com esse bit e faz-se o XOR.
func resto(valor, gerador, grauMax, grauGer int) int {
	for i := grauMax; i >= grauGer; i-- {
		if valor&(1<<i) != 0 {
			valor ^= gerador << (i - grauGer)
		}
	}
	return valor
}

// infoVersao calcula os 18 bits de informação de versão: 6 bits de versão
// seguidos de 12 bits de BCH(18,6), com gerador 0x1F25.
func infoVersao(versao int) int {
	d := versao << 12
	return d | resto(d, 0x1F25, 17, 12)
}

// infoFormato calcula os 15 bits de formato: 5 bits de nível e máscara, mais
// 10 bits de BCH(15,5) com gerador 0x537. O resultado leva XOR com a máscara
// fixa 0x5412 definida pela norma, para que o formato nunca fique todo zero.
func infoFormato(nivel Nivel, mascara int) int {
	dados := nivel.bits()<<3 | mascara
	d := dados << 10
	return (d | resto(d, 0x537, 14, 10)) ^ 0x5412
}

func (m *Matriz) desenharFormato(nivel Nivel, mascara int) {
	bits := infoFormato(nivel, mascara)
	for i := 0; i < 15; i++ {
		v := int8((bits >> i) & 1)

		// primeira cópia, em volta do finder superior esquerdo
		switch {
		case i < 6:
			m.setFuncao(8, i, v)
		case i == 6:
			m.setFuncao(8, 7, v)
		case i == 7:
			m.setFuncao(8, 8, v)
		case i == 8:
			m.setFuncao(7, 8, v)
		default:
			m.setFuncao(14-i, 8, v)
		}

		// segunda cópia, dividida entre os outros dois finder
		if i < 8 {
			m.setFuncao(8, m.tam-1-i, v)
		} else {
			m.setFuncao(m.tam-15+i, 8, v)
		}
	}
}

// preencher percorre a matriz em ziguezague, de baixo para cima, em colunas
// aos pares, da direita para a esquerda, pulando a coluna 6 do timing.
func (m *Matriz) preencher(dados []byte) {
	bit := 0
	subindo := true

	for direita := m.tam - 1; direita >= 1; direita -= 2 {
		if direita == 6 {
			direita = 5 // a coluna 6 é timing e não entra no par
		}
		for i := 0; i < m.tam; i++ {
			l := i
			if subindo {
				l = m.tam - 1 - i
			}
			for _, c := range []int{direita, direita - 1} {
				if m.ehFuncao(l, c) {
					continue
				}
				var v int8
				if bit < len(dados)*8 && dados[bit/8]&(1<<(7-bit%8)) != 0 {
					v = 1
				}
				m.set(l, c, v)
				bit++
			}
		}
		subindo = !subindo
	}
}

// aplicarMascara inverte os módulos de dados conforme a fórmula da máscara.
func (m *Matriz) aplicarMascara(mascara int) {
	for l := 0; l < m.tam; l++ {
		for c := 0; c < m.tam; c++ {
			if m.ehFuncao(l, c) {
				continue
			}
			var inverter bool
			switch mascara {
			case 0:
				inverter = (l+c)%2 == 0
			case 1:
				inverter = l%2 == 0
			case 2:
				inverter = c%3 == 0
			case 3:
				inverter = (l+c)%3 == 0
			case 4:
				inverter = (l/2+c/3)%2 == 0
			case 5:
				inverter = (l*c)%2+(l*c)%3 == 0
			case 6:
				inverter = ((l*c)%2+(l*c)%3)%2 == 0
			case 7:
				inverter = ((l+c)%2+(l*c)%3)%2 == 0
			}
			if inverter {
				m.set(l, c, m.get(l, c)^1)
			}
		}
	}
}

// penalidade avalia o quanto o desenho é ruim para leitura. A norma define
// quatro regras; a máscara escolhida é a de menor pontuação.
func (m *Matriz) penalidade() int {
	total := 0
	escuros := 0

	// regra 1: sequências de 5 ou mais módulos iguais, em linha e em coluna
	for i := 0; i < m.tam; i++ {
		for _, linha := range []bool{true, false} {
			cor, seq := int8(-1), 0
			for j := 0; j < m.tam; j++ {
				var v int8
				if linha {
					v = m.get(i, j)
				} else {
					v = m.get(j, i)
				}
				if v == cor {
					seq++
				} else {
					if seq >= 5 {
						total += seq - 2
					}
					cor, seq = v, 1
				}
			}
			if seq >= 5 {
				total += seq - 2
			}
		}
	}

	// regra 2: blocos 2x2 de mesma cor
	for l := 0; l < m.tam-1; l++ {
		for c := 0; c < m.tam-1; c++ {
			v := m.get(l, c)
			if v == m.get(l, c+1) && v == m.get(l+1, c) && v == m.get(l+1, c+1) {
				total += 3
			}
		}
	}

	// regra 3: padrão que o leitor confundiria com um finder
	alvo1 := []int8{1, 0, 1, 1, 1, 0, 1, 0, 0, 0, 0}
	alvo2 := []int8{0, 0, 0, 0, 1, 0, 1, 1, 1, 0, 1}
	casa := func(l, c int, linha bool, alvo []int8) bool {
		for k, esperado := range alvo {
			var v int8
			if linha {
				if c+k >= m.tam {
					return false
				}
				v = m.get(l, c+k)
			} else {
				if l+k >= m.tam {
					return false
				}
				v = m.get(l+k, c)
			}
			if v != esperado {
				return false
			}
		}
		return true
	}
	for l := 0; l < m.tam; l++ {
		for c := 0; c < m.tam; c++ {
			if casa(l, c, true, alvo1) || casa(l, c, true, alvo2) {
				total += 40
			}
			if casa(l, c, false, alvo1) || casa(l, c, false, alvo2) {
				total += 40
			}
		}
	}

	// regra 4: desequilíbrio entre claros e escuros
	for _, v := range m.celula {
		if v == 1 {
			escuros++
		}
	}
	proporcao := escuros * 100 / (m.tam * m.tam)
	desvio := proporcao - 50
	if desvio < 0 {
		desvio = -desvio
	}
	total += (desvio / 5) * 10

	return total
}

// ---------------------------------------------------------------- API

// Gerar monta o QR Code do texto e devolve a matriz pronta.
func Gerar(texto string, nivel Nivel) (*Matriz, int, error) {
	versao, err := escolherVersao(len(texto), nivel)
	if err != nil {
		return nil, 0, err
	}
	dados := codificarDados(texto, versao, nivel)
	tam := 21 + 4*(versao-1)

	melhorPontos := -1
	var melhor *Matriz

	// a norma manda testar as oito máscaras e ficar com a de menor penalidade
	for mascara := 0; mascara < 8; mascara++ {
		m := novaMatriz(tam)
		m.desenharPadroes(versao)
		m.preencher(dados)
		m.aplicarMascara(mascara)
		m.desenharFormato(nivel, mascara)

		if p := m.penalidade(); melhorPontos < 0 || p < melhorPontos {
			melhorPontos, melhor = p, m
		}
	}
	return melhor, versao, nil
}

// String desenha o QR em texto, útil para conferir no terminal.
func (m *Matriz) String() string {
	var b strings.Builder
	for l := 0; l < m.tam; l++ {
		for c := 0; c < m.tam; c++ {
			if m.get(l, c) == 1 {
				b.WriteString("██")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
