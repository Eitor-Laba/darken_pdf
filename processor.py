import fitz  # PyMuPDF
import os

def invert_via_pixmap(input_path, output_path):
    doc = fitz.open(input_path)
    new_doc = fitz.open()

    for page_num in range(len(doc)):
        page = doc.load_page(page_num)
        
        # Renderiza a página (ajuste Matrix para qualidade vs tamanho)
        pix = page.get_pixmap(matrix=fitz.Matrix(2, 2))
        
        # Lógica Manual de Inversão:
        # Se o pixmap for RGB (3 canais), subtraímos cada byte de 255
        # samples é um objeto 'bytes' mutável no Python ou acessível via bytearray
        img_data = bytearray(pix.samples)
        
        # Inverte apenas os canais de cor (ignora o canal Alpha se existir)
        # 255 - valor_atual = cor invertida
        for i in range(0, len(img_data), pix.n):
            img_data[i] = 255 - img_data[i]         # Red
            img_data[i+1] = 255 - img_data[i+1]     # Green
            img_data[i+2] = 255 - img_data[i+2]     # Blue
            # Se pix.n for 4, o quarto byte é o Alpha (transparência), não mexemos.

        # Cria um novo pixmap com os dados invertidos
        inverted_pix = fitz.Pixmap(pix.colorspace, pix.width, pix.height, img_data, pix.alpha)
        
        new_page = new_doc.new_page(width=page.rect.width, height=page.rect.height)
        new_page.insert_image(new_page.rect, pixmap=inverted_pix)

    new_doc.save(output_path, garbage=4, deflate=True)
    new_doc.close()
    doc.close()

def process_folder(folder_path="./pdf"):
    # Garante que o caminho seja absoluto ou relativo correto ao script
    if not os.path.exists(folder_path):
        os.makedirs(folder_path)
        print(f"Pasta {folder_path} criada.")
        return

    files = [f for f in os.listdir(folder_path) if f.endswith(".pdf") and not f.endswith("_dark.pdf")]
    
    for filename in files:
        input_path = os.path.join(folder_path, filename)
        output_path = os.path.join(folder_path, filename.replace(".pdf", "_dark.pdf"))

        print(f"Processando: {filename}...")
        try:
            invert_via_pixmap(input_path, output_path)
            print(f"Sucesso!")
        except Exception as e:
            print(f"Erro: {e}")

if __name__ == "__main__":
    process_folder()