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

	tempIn, _ := os.CreateTemp("", "input-*.pdf")
	defer os.Remove(tempIn.Name())
	io.Copy(tempIn, file)

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

	numPages := doc.NumPage()
	pages := make([]image.Image, numPages)

	// Canal para coletar resultados e WaitGroup para sincronia
	resChan := make(chan pageData, numPages)
	var wg sync.WaitGroup

	// Limitador de concorrência (usa o número de CPUs disponíveis)
	semaphore := make(chan struct{}, runtime.NumCPU())

	for i := 0; i < numPages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}        // Adquire slot
			defer func() { <-semaphore }() // Libera slot

			img, err := doc.Image(idx)
			if err == nil {
				inverted := fastInvert(img)
				resChan <- pageData{index: idx, img: inverted}
			}
		}(i)
	}

	// Fecha o canal quando terminar de processar
	go func() {
		wg.Wait()
		close(resChan)
	}()

	// Coleta os resultados mantendo a ordem original
	for p := range resChan {
		pages[p.index] = p.img
	}

	// Reconstrói o PDF
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	for _, img := range pages {
		if img == nil {
			continue
		}

		// Detecta orientação original da página processada
		rect := img.Bounds()
		w, h := float64(rect.Dx()), float64(rect.Dy())

		pdf.AddPageWithOption(gopdf.PageOption{
			PageSize: &gopdf.Rect{W: w, H: h},
		})

		_ = pdf.ImageFrom(img, 0, 0, nil)
	}

	var buf bytes.Buffer
	pdf.WriteTo(&buf)
	return &buf, nil
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
