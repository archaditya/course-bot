from typing import List

import httpx
from bs4 import BeautifulSoup
from pydantic import BaseModel


class URLSection(BaseModel):
    text: str
    heading: str | None = None


class URLExtractionResult(BaseModel):
    title: str
    sections: List[URLSection]


class URLExtractorService:
    """Fetches a web page and extracts readable text content.

    Uses BeautifulSoup with a readability-style extraction approach:
    strips nav, footer, sidebar, ads, and other non-content elements,
    then segments the remaining text by headings/paragraphs.
    """

    # Elements that typically contain navigation, ads, or non-content
    NOISE_TAGS = {"nav", "footer", "header", "aside", "script", "style", "noscript", "iframe"}
    NOISE_CLASSES = {"sidebar", "navigation", "nav", "footer", "header", "ad", "advertisement", "menu", "cookie"}

    async def extract(self, url: str) -> URLExtractionResult:
        async with httpx.AsyncClient(timeout=30.0, follow_redirects=True) as client:
            response = await client.get(url, headers={
                "User-Agent": "archadiLM/1.0 (course-material-indexer)"
            })
            response.raise_for_status()

        soup = BeautifulSoup(response.text, "html.parser")

        # Remove noise elements
        for tag_name in self.NOISE_TAGS:
            for tag in soup.find_all(tag_name):
                tag.decompose()

        for tag in soup.find_all(attrs={"class": True}):
            classes = " ".join(tag.get("class", [])).lower()
            if any(noise in classes for noise in self.NOISE_CLASSES):
                tag.decompose()

        title = soup.title.string.strip() if soup.title and soup.title.string else url

        # Extract text by sections (headings + their content)
        sections: List[URLSection] = []
        current_heading = None
        current_text: list[str] = []

        content_area = soup.find("main") or soup.find("article") or soup.body
        if not content_area:
            return URLExtractionResult(title=title, sections=[])

        for element in content_area.descendants:
            if element.name in {"h1", "h2", "h3", "h4", "h5", "h6"}:
                # Flush previous section
                if current_text:
                    text = " ".join(current_text).strip()
                    if text:
                        sections.append(URLSection(text=text, heading=current_heading))
                current_heading = element.get_text(strip=True)
                current_text = []
            elif element.name in {"p", "li", "td", "blockquote", "pre", "code"}:
                text = element.get_text(strip=True)
                if text and len(text) > 10:  # filter out tiny fragments
                    current_text.append(text)

        # Flush last section
        if current_text:
            text = " ".join(current_text).strip()
            if text:
                sections.append(URLSection(text=text, heading=current_heading))

        return URLExtractionResult(title=title, sections=sections)

