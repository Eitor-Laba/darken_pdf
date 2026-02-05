import os
import tempfile
from flask import Flask, request, send_file, send_from_directory, abort

from processor import invert_via_pixmap

app = Flask(__name__, static_folder="public", static_url_path="")

EXPECTED_TOKEN = "a06410e3c0031514891e8d1eaf0be6f20a253912"


@app.get("/")
def index():
    return send_from_directory(app.static_folder, "index.html")


@app.post("/convert")
def convert():
    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Bearer "):
        return "Token ausente", 401

    token = auth_header.removeprefix("Bearer ").strip()
    if token != EXPECTED_TOKEN:
        return "Token inválido", 403

    if "pdf" not in request.files:
        return "Arquivo PDF ausente", 400

    uploaded = request.files["pdf"]
    if uploaded.filename == "":
        return "Arquivo inválido", 400

    base_name = os.path.basename(uploaded.filename)
    name_root, ext = os.path.splitext(base_name)
    if not ext:
        ext = ".pdf"
    output_filename = f"{name_root}_dark{ext}"

    with tempfile.TemporaryDirectory() as temp_dir:
        input_path = os.path.join(temp_dir, "input.pdf")
        output_path = os.path.join(temp_dir, output_filename)

        uploaded.save(input_path)
        try:
            invert_via_pixmap(input_path, output_path)
        except Exception as exc:
            return f"Falha no processamento: {exc}", 500

        if not os.path.exists(output_path):
            return "Falha ao gerar PDF", 500

        return send_file(
            output_path,
            mimetype="application/pdf",
            as_attachment=True,
            download_name=output_filename,
        )


@app.get("/<path:filename>")
def static_files(filename: str):
    file_path = os.path.join(app.static_folder, filename)
    if not os.path.isfile(file_path):
        abort(404)
    return send_from_directory(app.static_folder, filename)


if __name__ == "__main__":
    port = int(os.getenv("PORT", "8080"))
    app.run(host="0.0.0.0", port=port)
