// qrpix monta o BR Code do PIX e gera a imagem do QR Code.
//
// Tudo acontece nesta máquina: o payload é montado aqui, o QR Code é desenhado
// aqui e o PNG é gravado aqui. A chave PIX não é enviada para lugar nenhum.
//
//	go run ./tools/qrpix -chave "email@exemplo.com" -nome "FULANO" -cidade "CIDADE" -saida qr.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"unicode"
)

func main() {
	var (
		chave  = flag.String("chave", "", "chave PIX (obrigatório)")
		nome   = flag.String("nome", "", "nome do recebedor (obrigatório)")
		cidade = flag.String("cidade", "", "cidade do recebedor (obrigatório)")
		texto  = flag.String("texto", "", "gera o QR de um texto qualquer, ignorando os campos do PIX")
		saida  = flag.String("saida", "qrcode.png", "arquivo PNG de saída")
		escala = flag.Int("escala", 8, "pixels por módulo")
		margem = flag.Int("margem", 4, "margem em módulos (a norma pede 4)")
		mostrar = flag.Bool("mostrar", false, "desenha o QR no terminal")
	)
	flag.Parse()

	conteudo := *texto
	if conteudo == "" {
		if *chave == "" || *nome == "" || *cidade == "" {
			fmt.Fprintln(os.Stderr, "informe -chave, -nome e -cidade (ou use -texto)")
			flag.Usage()
			os.Exit(2)
		}
		conteudo = MontarBRCode(*chave, *nome, *cidade)
	}

	m, versao, err := Gerar(conteudo, NivelM)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}

	if err := gravarPNG(m, *saida, *escala, *margem); err != nil {
		fmt.Fprintln(os.Stderr, "erro ao gravar:", err)
		os.Exit(1)
	}

	if *mostrar {
		fmt.Print(m.String())
	}

	fmt.Println()
	fmt.Println("payload :", conteudo)
	fmt.Printf("QR Code : versão %d, %dx%d módulos, correção de erro nível M\n", versao, m.tam, m.tam)
	fmt.Println("imagem  :", *saida)
	fmt.Println()
	fmt.Println("Confira no app do banco antes de publicar: escaneie a imagem e veja")
	fmt.Println("se aparece o recebedor correto.")
	fmt.Println()
}

// ---------------------------------------------------------------- BR Code

// campo monta um elemento no formato EMV: identificador, tamanho e valor.
func campo(id, valor string) string {
	return fmt.Sprintf("%s%02d%s", id, len(valor), valor)
}

// normalizar deixa o texto no ASCII maiúsculo que o padrão aceita.
func normalizar(t string, max int) string {
	var b strings.Builder
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(unicode.ToUpper(r))
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ':
			b.WriteRune(r)
		default:
			// acentos viram a letra base; o resto é descartado
			if s, ok := semAcento[r]; ok {
				b.WriteString(s)
			}
		}
	}
	s := strings.Join(strings.Fields(b.String()), " ")
	if len(s) > max {
		s = strings.TrimSpace(s[:max])
	}
	return s
}

var semAcento = map[rune]string{
	'á': "A", 'à': "A", 'ã': "A", 'â': "A", 'ä': "A",
	'é': "E", 'ê': "E", 'è': "E", 'ë': "E",
	'í': "I", 'î': "I", 'ì': "I", 'ï': "I",
	'ó': "O", 'ô': "O", 'õ': "O", 'ò': "O", 'ö': "O",
	'ú': "U", 'û': "U", 'ù': "U", 'ü': "U",
	'ç': "C", 'ñ': "N",
	'Á': "A", 'À': "A", 'Ã': "A", 'Â': "A", 'Ä': "A",
	'É': "E", 'Ê': "E", 'È': "E", 'Ë': "E",
	'Í': "I", 'Î': "I", 'Ì': "I", 'Ï': "I",
	'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ò': "O", 'Ö': "O",
	'Ú': "U", 'Û': "U", 'Ù': "U", 'Ü': "U",
	'Ç': "C", 'Ñ': "N",
}

// MontarBRCode gera o "Pix Copia e Cola" no padrão EMV do Banco Central.
func MontarBRCode(chave, nome, cidade string) string {
	conta := campo("00", "br.gov.bcb.pix") + campo("01", chave)

	p := campo("00", "01") // versão do payload
	p += campo("26", conta)
	p += campo("52", "0000") // categoria do recebedor
	p += campo("53", "986")  // moeda: real
	p += campo("58", "BR")
	p += campo("59", normalizar(nome, 25))
	p += campo("60", normalizar(cidade, 15))
	p += campo("62", campo("05", "***"))
	p += "6304" // o cabeçalho do CRC entra no próprio cálculo

	return p + crc16(p)
}

// crc16 implementa o CRC16/CCITT-FALSE exigido pelo Banco Central.
func crc16(s string) string {
	crc := uint16(0xFFFF)
	for _, b := range []byte(s) {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}

// ---------------------------------------------------------------- imagem

func gravarPNG(m *Matriz, caminho string, escala, margem int) error {
	lado := (m.tam + margem*2) * escala
	img := image.NewGray(image.Rect(0, 0, lado, lado))

	branco := color.Gray{Y: 255}
	preto := color.Gray{Y: 0}

	// a margem clara faz parte da especificação: sem ela muitos leitores falham
	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			img.SetGray(x, y, branco)
		}
	}
	for l := 0; l < m.tam; l++ {
		for c := 0; c < m.tam; c++ {
			if m.get(l, c) != 1 {
				continue
			}
			for y := 0; y < escala; y++ {
				for x := 0; x < escala; x++ {
					img.SetGray((c+margem)*escala+x, (l+margem)*escala+y, preto)
				}
			}
		}
	}

	f, err := os.Create(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
