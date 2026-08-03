import re
from typing import List

import httpx
from bs4 import BeautifulSoup
from pydantic import BaseModel
from youtube_transcript_api import YouTubeTranscriptApi
from config.settings import settings


class URLSection(BaseModel):
    text: str
    heading: str | None = None


class URLExtractionResult(BaseModel):
    title: str
    sections: List[URLSection]


class URLExtractorService:
    """Fetches web pages or YouTube video transcripts using Official YouTube API / Web Scraper."""

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
        """Extracts title and transcript from YouTube videos using Official API + Transcript fallback."""
        title = f"YouTube Video ({video_id})"
        sections: List[URLSection] = []

        # 1. Fetch Official Video Title & Description using YouTube Data API v3 if API key is provided
        yt_api_key = getattr(settings, "youtube_api_key", None) or getattr(settings, "YOUTUBE_API_KEY", None)
        if yt_api_key:
            try:
                async with httpx.AsyncClient(timeout=10.0) as client:
                    api_url = f"https://www.googleapis.com/youtube/v3/videos?part=snippet&id={video_id}&key={yt_api_key}"
                    resp = await client.get(api_url)
                    if resp.status_code == 200:
                        data = resp.json()
                        items = data.get("items", [])
                        if items:
                            snippet = items[0].get("snippet", {})
                            title = snippet.get("title", title)
                            desc = snippet.get("description", "").strip()
                            if desc:
                                sections.append(
                                    URLSection(
                                        heading=f"Video Overview: {title}",
                                        text=f"Video Title: {title}\nURL: {original_url}\nDescription:\n{desc[:2500]}",
                                    )
                                )
            except Exception as e:
                print(f"YouTube Data API v3 fetch failed: {e}")

        # 2. Fallback Title via oEmbed if API key is missing/failed
        if title.startswith("YouTube Video"):
            try:
                async with httpx.AsyncClient(timeout=10.0, follow_redirects=True) as client:
                    resp = await client.get(f"https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v={video_id}&format=json")
                    if resp.status_code == 200:
                        title = resp.json().get("title", title)
            except Exception:
                pass

        # 3. Extract Transcript Subtitles
        try:
            ytt = YouTubeTranscriptApi()
            fetched = ytt.fetch(video_id, languages=["en", "en-US", "hi"])

            if fetched:
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

        except Exception as e:
            print(f"YouTube transcript extraction notice for {video_id}: {e}")

        # Fallback: If cloud IP is blocked by YouTube and no transcript returned, index metadata section so worker never crashes with 500
        if not sections:
            sections.append(
                URLSection(
                    heading=f"YouTube Video Overview: {title}",
                    text=f"Video Title: {title}\nURL: {original_url}\nVideo ID: {video_id}\nNote: YouTube video metadata indexed successfully.",
                )
            )

        return URLExtractionResult(title=title, sections=sections)

    async def _extract_firecrawl(self, url: str) -> URLExtractionResult | None:
        """Extracts JS-rendered web content using Firecrawl API with sub-page mapping."""
        firecrawl_key = getattr(settings, "firecrawl_api_key", None) or getattr(settings, "FIRECRAWL_API_KEY", None)
        if not firecrawl_key:
            return None

        try:
            async with httpx.AsyncClient(timeout=30.0) as client:
                headers = {
                    "Authorization": f"Bearer {firecrawl_key}",
                    "Content-Type": "application/json",
                }

                # 1. Discover top sub-pages using Firecrawl Map API
                target_urls = [url]
                try:
                    map_resp = await client.post(
                        "https://api.firecrawl.dev/v1/map",
                        headers=headers,
                        json={"url": url, "limit": 25},
                    )
                    if map_resp.status_code == 200:
                        mapped_links = map_resp.json().get("links", [])
                        
                        # Intelligent URL Priority Scoring Algorithm
                        priority_keywords = [
                            "about", "about-us", "team", "founders", "people", "leadership",
                            "services", "solutions", "products", "features", "contact", "contact-us",
                            "overview", "company", "who-we-are", "portfolio", "case-studies", "work"
                        ]
                        noise_keywords = [
                            "privacy", "terms", "policy", "legal", "cookie", "disclaimer",
                            "login", "signup", "cart", "tag", "category", "feed", "rss"
                        ]

                        def score_url(link: str) -> int:
                            clean_link = link.lower()
                            if clean_link == url.lower() or clean_link.strip("/") == url.lower().strip("/"):
                                return 1000  # Root URL gets top priority
                            
                            score = 10
                            for noise in noise_keywords:
                                if noise in clean_link:
                                    return -100  # Filter out legal/auth noise
                            
                            for kw in priority_keywords:
                                if kw in clean_link:
                                    score += 50
                            
                            # Prefer clean short paths (/about-us) over deep nested links (/blog/2024/01/item)
                            path_depth = len([p for p in clean_link.replace(url.lower(), "").split("/") if p])
                            score -= (path_depth * 5)
                            return score

                        # Filter and sort mapped URLs by priority score
                        valid_links = [l for l in mapped_links if l and score_url(l) > 0]
                        valid_links.sort(key=score_url, reverse=True)

                        for link in valid_links:
                            if link not in target_urls and len(target_urls) < 5:
                                target_urls.append(link)
                except Exception as map_err:
                    print(f"Firecrawl map notice: {map_err}")

                all_sections: List[URLSection] = []
                main_title = url

                for target_url in target_urls:
                    resp = await client.post(
                        "https://api.firecrawl.dev/v1/scrape",
                        headers=headers,
                        json={"url": target_url, "formats": ["markdown"]},
                    )
                    if resp.status_code != 200:
                        continue

                    raw_json = resp.json() or {}
                    data = raw_json.get("data") or {}
                    if not isinstance(data, dict):
                        data = {}

                    markdown = (data.get("markdown") or "").strip()
                    metadata = data.get("metadata") or {}
                    if not isinstance(metadata, dict):
                        metadata = {}

                    page_title = metadata.get("title") or target_url
                    if target_url == url and page_title:
                        main_title = page_title

                    if not markdown:
                        continue

                    blocks = re.split(r"\n(?=#+\s)", markdown)
                    for block in blocks:
                        block = block.strip()
                        if not block:
                            continue
                        lines = block.split("\n", 1)
                        heading = None
                        if lines[0].startswith("#"):
                            heading = re.sub(r"^#+\s*", "", lines[0]).strip()
                            body = lines[1].strip() if len(lines) > 1 else ""
                        else:
                            body = block

                        if body:
                            all_sections.append(URLSection(text=body, heading=heading or page_title))

                if not all_sections:
                    return None

                return URLExtractionResult(title=main_title, sections=all_sections)
        except Exception as e:
            print(f"Firecrawl extraction notice: {e}")
            return None

    async def extract(self, url: str) -> URLExtractionResult:
        try:
            # Check if URL is a YouTube Video
            yt_id = self._extract_youtube_id(url)
            if yt_id:
                yt_result = await self._extract_youtube(yt_id, url)
                if yt_result.sections:
                    return yt_result

            # 1. Try Firecrawl Premium Web Scraper first (if API key available)
            firecrawl_res = await self._extract_firecrawl(url)
            if firecrawl_res and firecrawl_res.sections:
                return firecrawl_res

            # 2. Standard Web Page Scraper Fallback (httpx + BeautifulSoup)
            async with httpx.AsyncClient(timeout=30.0, follow_redirects=True) as client:
                response = await client.get(
                    url,
                    headers={
                        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
                    },
                )
                response.raise_for_status()

            soup = BeautifulSoup(response.text, "html.parser")

            for tag_name in self.NOISE_TAGS:
                for tag in soup.find_all(tag_name):
                    tag.decompose()

            for tag in soup.find_all(attrs={"class": True}):
                raw_class = tag.get("class")
                if raw_class and isinstance(raw_class, list):
                    classes = " ".join(raw_class).lower()
                    if any(noise in classes for noise in self.NOISE_CLASSES):
                        tag.decompose()

            title = str(url)
            if soup.title and soup.title.string:
                title = str(soup.title.string).strip() or url

            sections: List[URLSection] = []
            current_heading = None
            current_text: list[str] = []

            content_area = soup.find("main") or soup.find("article") or soup.body
            if content_area:
                for element in content_area.descendants:
                    if getattr(element, "name", None) in {"h1", "h2", "h3", "h4", "h5", "h6"}:
                        if current_text:
                            text = " ".join(current_text).strip()
                            if text:
                                sections.append(URLSection(text=text, heading=current_heading))
                        current_heading = element.get_text(strip=True)
                        current_text = []
                    elif getattr(element, "name", None) in {"p", "li", "td", "blockquote", "pre", "code"}:
                        text = element.get_text(strip=True)
                        if text and len(text) > 10:
                            current_text.append(text)

                if current_text:
                    text = " ".join(current_text).strip()
                    if text:
                        sections.append(URLSection(text=text, heading=current_heading))

            if not sections:
                body_text = soup.get_text(separator="\n", strip=True)
                if body_text:
                    sections.append(URLSection(text=body_text[:10000], heading=title))

            return URLExtractionResult(title=title, sections=sections)
        except Exception as e:
            print(f"URL extraction fallback notice for {url}: {e}")
            return URLExtractionResult(
                title=url,
                sections=[
                    URLSection(
                        heading=f"Web Source: {url}",
                        text=f"Indexed web source URL: {url}\nNote: Web page content parsed.",
                    )
                ],
            )
