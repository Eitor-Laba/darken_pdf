package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/gen2brain/go-fitz"
	"github.com/signintech/gopdf"
	xdraw "golang.org/x/image/draw"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/convert", handleUpload)
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	port := ":8080"
	fmt.Printf("🚀 Servidor Dark Mode rodando em http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Apenas POST permitido", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("pdf")
	if err != nil {
		http.Error(w, "Erro no arquivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempIn, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		http.Error(w, "Erro ao criar arquivo temporário", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempIn.Name())
	if _, err := io.Copy(tempIn, file); err != nil {
		http.Error(w, "Erro ao salvar arquivo", http.StatusInternalServerError)
		return
	}
	if err := tempIn.Close(); err != nil {
		http.Error(w, "Erro ao finalizar arquivo", http.StatusInternalServerError)
		return
	}

	outputBuffer, err := processAndInvert(tempIn.Name())
	if err != nil {
		http.Error(w, "Falha no processamento: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=dark_mode.pdf")
	w.Write(outputBuffer.Bytes())
}

type pageData struct {
	index int
	img   image.Image
}

func processAndInvert(tempPath string) (*bytes.Buffer, error) {
	doc, err := fitz.New(tempPath)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	// --- CONFIGURAÇÃO DE QUALIDADE ---
	// 0.5 = Baixa (Rápido, arquivo pequeno, texto serrilhado) ~36 DPI
	// 1.0 = Padrão (Normal, leitura ok) ~72 DPI
	// 2.0 = Alta (Texto nítido, arquivo grande) ~144 DPI
	// 3.0 = Impressão (Arquivo GIGANTE) ~216 DPI
	const scaleFactor = 1.5
	// ---------------------------------

	numPages := doc.NumPage()
	if numPages == 0 {
		return nil, fmt.Errorf("pdf sem páginas")
	}
	pages := make([]image.Image, numPages)

	resChan := make(chan pageData, numPages)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// Limita concorrência para evitar estouro de RAM com imagens grandes
	semaphore := make(chan struct{}, runtime.NumCPU())

	for i := 0; i < numPages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			baseImg, err := doc.Image(idx)
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}

			var img image.Image = baseImg
			if scaleFactor != 1 {
				img = scaleImage(img, scaleFactor)
			}
			inverted := fastInvert(img)
			resChan <- pageData{index: idx, img: inverted}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	for p := range resChan {
		pages[p.index] = p.img
	}

	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	// Reconstrói o PDF
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	for _, img := range pages {
		if img == nil {
			continue
		}

		// As dimensões retornadas pelo ImageScale são em Pixels.
		// O gopdf usa Pontos (Points) por padrão (1 pt = 1/72 inch).
		// Precisamos ajustar o tamanho da página do PDF para bater com a imagem escalada,
		// ou a imagem vai parecer "gigante" ou "minúscula" se não ajustarmos.

		// Truque: O gopdf aceita o tamanho em pontos.
		// Se aumentamos a escala da imagem (ex: 2.0), ela tem mais pixels.
		// Para ela caber na página visualmente igual a original, definimos a página
		// com o tamanho dos pixels da imagem.
		rect := img.Bounds()
		w, h := float64(rect.Dx()), float64(rect.Dy())

		pdf.AddPageWithOption(gopdf.PageOption{
			PageSize: &gopdf.Rect{W: w, H: h},
		})

		// Desenha a imagem ocupando toda a área da página
		if err := pdf.ImageFrom(img, 0, 0, nil); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return &buf, nil
}

func scaleImage(src image.Image, factor float64) image.Image {
	bounds := src.Bounds()
	if factor <= 0 {
		return src
	}

	newW := int(float64(bounds.Dx()) * factor)
	newH := int(float64(bounds.Dy()) * factor)
	if newW <= 0 || newH <= 0 {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}

// fastInvert usa manipulação direta de memória para ser ultra rápido
func fastInvert(img image.Image) image.Image {
	bounds := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		// Fallback se não for RGBA (raro com go-fitz)
		newRgba := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
				newRgba.Set(x, y, color.RGBA{255 - c.R, 255 - c.G, 255 - c.B, c.A})
			}
		}
		return newRgba
	}

	// Inversão direta no slice de pixels (muito mais rápido)
	inverted := image.NewRGBA(bounds)
	copy(inverted.Pix, rgba.Pix)

	for i := 0; i < len(inverted.Pix); i += 4 {
		inverted.Pix[i] = 255 - inverted.Pix[i]     // R
		inverted.Pix[i+1] = 255 - inverted.Pix[i+1] // G
		inverted.Pix[i+2] = 255 - inverted.Pix[i+2] // B
		// Pix[i+3] é o Alpha, mantemos original
	}

	return inverted
}
