import json
import logging
from typing import List

from app.providers import LLMProvider
from app.providers.base import Prompt

logger = logging.getLogger("ai-service.source-intel")


class SourceIntelResult:
    """Structured result from a single source intelligence generation call."""

    def __init__(self, summary: str, questions: List[str], overview: str):
        self.summary = summary
        self.questions = questions
        self.overview = overview


class SourceIntelService:
    """Generates NotebookLM-style source intelligence: summary, suggested
    questions, and a welcome overview — all in a single LLM call."""

    def __init__(self, provider: LLMProvider):
        self.provider = provider

    async def generate_intel(
        self, content: str, filename: str, prompt_version: str = "1.0"
    ) -> SourceIntelResult:
        prompt = Prompt(
            text=f"""Analyze the following source content and return a JSON object with exactly 3 keys:

1. "summary" — A 2-paragraph plain-text Source Guide. Paragraph 1 describes the core topic, teaching style, and audience. Paragraph 2 covers key technical concepts, tools, and takeaways. No markdown formatting.

2. "questions" — An array of 5 specific, interesting questions a student would ask about this content. Each question should reference actual topics, concepts, or examples found in the material. Never generate generic questions like "What are the key concepts?".

3. "overview" — A 2-3 sentence friendly welcome message telling the user what this source contains and what they can ask about. Write it like a study partner greeting, e.g. "This source covers X, Y, and Z. You can ask me about..."

Source filename: {filename}

Content excerpt:
{content[:6000]}""",
            system_prompt="You are a technical content analyst. Return valid JSON only. No markdown, no code fences.",
            temperature=0.3,
            max_tokens=600,
            prompt_version=prompt_version,
        )

        response = await self.provider.generate(prompt)
        raw = response.text.strip()

        # Strip markdown code fences if the LLM wraps the JSON
        if raw.startswith("```"):
            raw = raw.split("\n", 1)[-1]
        if raw.endswith("```"):
            raw = raw.rsplit("```", 1)[0]
        raw = raw.strip()

        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            logger.warning("Source intel JSON parse failed, falling back to raw text")
            return SourceIntelResult(
                summary=raw[:500],
                questions=[],
                overview=f"This source ({filename}) has been indexed and is ready for questions.",
            )

        return SourceIntelResult(
            summary=data.get("summary", ""),
            questions=data.get("questions", [])[:5],
            overview=data.get("overview", ""),
        )

    # Backward-compatible wrapper for existing callers
    async def generate_summary(self, content: str, prompt_version: str = "1.0") -> str:
        result = await self.generate_intel(content, "unknown", prompt_version)
        return result.summary
