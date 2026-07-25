import re
from typing import List

import httpx
from bs4 import BeautifulSoup
from pydantic import BaseModel
from youtube_transcript_api import YouTubeTranscriptApi


class URLSection(BaseModel):
    text: str
    heading: str | None = None


class URLExtractionResult(BaseModel):
    title: str
    sections: List[URLSection]


class URLExtractorService:
    """Fetches a web page or YouTube video transcript and extracts readable text content."""

    NOISE_TAGS = {"nav", "footer", "header", "aside", "script", "style", "noscript", "iframe"}
    NOISE_CLASSES = {"sidebar", "navigation", "nav", "footer", "header", "ad", "advertisement", "menu", "cookie"}

    def _extract_youtube_id(self, url: str) -> str | None:
        """Extracts YouTube video ID from various URL formats."""
        patterns = [
            r"(?:v=|\/)([a-zA-Z0-9_-]{11})(?:[\?&]|$)",
            r"youtu\.be\/([a-zA-Z0-9_-]{11})",
            r"youtube\.com\/embed\/([a-zA-Z0-9_-]{11})",
        ]
        for pattern in patterns:
            match = re.search(pattern, url)
            if match:
                return match.group(1)
        return None

    async def _extract_youtube(self, video_id: str, original_url: str) -> URLExtractionResult:
        """Extracts title and timestamped transcript from YouTube videos using YouTubeTranscriptApi."""
        title = f"YouTube Video ({video_id})"

        # 1. Try fetching video title via YouTube oEmbed API
        try:
            async with httpx.AsyncClient(timeout=10.0, follow_redirects=True) as client:
                resp = await client.get(f"https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v={video_id}&format=json")
                if resp.status_code == 200:
                    data = resp.json()
                    title = data.get("title", title)
        except Exception:
            pass

        # 2. Extract transcript using YouTubeTranscriptApi instance
        try:
            ytt = YouTubeTranscriptApi()
            fetched = ytt.fetch(video_id, languages=["en", "en-US", "hi"])

            if fetched:
                sections: List[URLSection] = []
                chunk_text: List[str] = []
                chunk_start_ts = "00:00"

                for i, item in enumerate(fetched):
                    start_sec = int(getattr(item, "start", 0))
                    mins = start_sec // 60
                    secs = start_sec % 60
                    timestamp = f"{mins:02d}:{secs:02d}"

                    if not chunk_text:
                        chunk_start_ts = timestamp

                    text_str = getattr(item, "text", "").strip()
                    if text_str:
                        chunk_text.append(text_str)

                    # Group transcript into ~800 character sections with timestamp headers
                    if len(" ".join(chunk_text)) > 800 or i == len(fetched) - 1:
                        section_content = " ".join(chunk_text).strip()
                        if section_content:
                            sections.append(
                                URLSection(
                                    heading=f"Transcript Segment [{chunk_start_ts}]",
                                    text=f"[{chunk_start_ts}] {section_content}",
                                )
                            )
                        chunk_text = []

                if sections:
                    return URLExtractionResult(title=title, sections=sections)

        except Exception as e:
            print(f"YouTube transcript extraction failed for {video_id}: {e}")

        return URLExtractionResult(title=title, sections=[])

    async def extract(self, url: str) -> URLExtractionResult:
        # Check if URL is a YouTube Video
        yt_id = self._extract_youtube_id(url)
        if yt_id:
            yt_result = await self._extract_youtube(yt_id, url)
            if yt_result.sections:
                return yt_result

        # Standard Web Page Scraper Fallback
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

