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

	"github.com/gen2brain/go-fitz"
	"github.com/signintech/gopdf"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/convert", handleUpload)
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	port := ":8080"
	fmt.Printf("Servidor rodando em http://localhost%s/convert\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Apenas POST é permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Recebe o arquivo do form-data
	file, _, err := r.FormFile("pdf")
	if err != nil {
		http.Error(w, "Erro ao ler arquivo do formulário", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. Salva temporariamente para o fitz conseguir ler
	tempIn, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempIn.Name())
	io.Copy(tempIn, file)

	// 3. Processa e inverte
	outputBuffer, err := processAndInvert(tempIn.Name())
	if err != nil {
		http.Error(w, "Erro ao processar PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Devolve o arquivo para o usuário
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=dark_mode.pdf")
	w.Write(outputBuffer.Bytes())
}

func processAndInvert(tempPath string) (*bytes.Buffer, error) {
	doc, err := fitz.New(tempPath)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4}) // Tamanho base, será ajustado por página

	for i := 0; i < doc.NumPage(); i++ {
		img, err := doc.Image(i)
		if err != nil {
			return nil, err
		}

		// Inverte as cores
		inverted := invertImage(img)

		// Adiciona página ao novo PDF com as dimensões da imagem
		w, h := float64(inverted.Bounds().Dx()), float64(inverted.Bounds().Dy())
		pdf.AddPageWithOption(gopdf.PageOption{PageSize: &gopdf.Rect{W: w, H: h}})

		// Insere a imagem na página
		err = pdf.ImageFrom(inverted, 0, 0, nil)
		if err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	pdf.WriteTo(&buf)
	return &buf, nil
}

func invertImage(img image.Image) image.Image {
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			inverted.Set(x, y, color.RGBA{255 - c.R, 255 - c.G, 255 - c.B, c.A})
		}
	}
	return inverted
}
