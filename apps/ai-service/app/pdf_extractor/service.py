import io
from typing import List

from pydantic import BaseModel


class PDFPage(BaseModel):
    page_number: int
    text: str


class PDFExtractorService:
    """Extracts text page-by-page from a PDF using PyMuPDF (fitz).

    PyMuPDF was chosen over alternatives because:
    - No system dependencies (pure wheel, no poppler/tesseract needed)
    - Handles both native text PDFs and scanned PDFs (with OCR fallback)
    - Preserves layout and table structure better than pdfplumber for most cases
    """

    async def extract(self, pdf_bytes: bytes) -> List[PDFPage]:
        import fitz  # PyMuPDF

        pages = []
        with fitz.open(stream=pdf_bytes, filetype="pdf") as doc:
            for page_num, page in enumerate(doc, start=1):
                text = page.get_text("text")
                if text.strip():
                    pages.append(PDFPage(page_number=page_num, text=text.strip()))
        return pages
