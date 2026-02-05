# 🌑 PDF Dark Mode Service (Golang)

This is a micro-service written in Go that inverts the colors of PDF files, turning white backgrounds to black and black text to white. Ideal for night reading or reducing eye strain.

The service works by processing each PDF page as a high-resolution image, inverting the color channels (RGB), and reconstructing the document.

## 🚀 Features

- **Total Inversion**: Transforms light PDFs into real Dark Mode.
- **HTTP API**: Simple endpoint for immediate upload and download.
- **High Performance**: Native Go pixel processing.
- **Security**: Uses temporary files that are removed after processing.

## 🛠️ Prerequisites

To run this project, you will need to have installed:

- **Go** (version 1.20 or higher)
- **MuPDF Core Library** (required for the `go-fitz` dependency)
    - **Windows**: Generally, `go-fitz` downloads the necessary binaries automatically.
    - **Linux**: You may need to install packages like `libmupdf-dev`.
