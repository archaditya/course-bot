import json
import logging
from typing import Any, Dict

from app.providers import LLMProvider
from app.providers.base import Prompt

logger = logging.getLogger("ai-service.learning-tool")


class LearningToolService:
    """Generates NotebookLM-style study artifacts: Summary, Key Takeaways, Flashcards,
    Quiz, Mind Map, or AI Report."""

    def __init__(self, provider: LLMProvider):
        self.provider = provider

    async def generate(self, tool_type: str, content: str, title: str = "") -> Any:
        content_excerpt = content[:15000]

        if tool_type == "summary":
            prompt_text = f"""Analyze the following source material and generate a comprehensive Summary.

Title: {title or 'Source Summary'}
Content:
{content_excerpt}

Return a JSON object with key "summary" containing markdown formatted text:
Use ## Headings, **bold terms**, and clean bullet points to summarize core concepts, key takeaways, and conclusions."""

        elif tool_type == "key_takeaways":
            prompt_text = f"""Analyze the following source material and generate Key Takeaways.

Title: {title or 'Key Takeaways'}
Content:
{content_excerpt}

Return a JSON object with key "takeaways" containing markdown formatted text:
Use numbered lists (1. 2. 3.) and bullet points to highlight actionable insights, key definitions, and critical takeaways."""

        elif tool_type == "flashcards":
            prompt_text = f"""Generate a set of 8 to 12 flashcards based on the following source material for active recall study.

Title: {title or 'Study Flashcards'}
Content:
{content_excerpt}

Return a JSON object with key "cards" which is an array of objects:
[
  {{"front": "Question or Key Concept?", "back": "Concise, accurate answer or explanation."}}
]"""

        elif tool_type == "quiz":
            prompt_text = f"""Generate a multiple-choice Quiz of 6 to 8 questions based on the following source material.

Title: {title or 'Knowledge Quiz'}
Content:
{content_excerpt}

Return a JSON object with key "quiz" which is an array of question objects:
[
  {{
    "question": "Question text?",
    "options": ["A) Option 1", "B) Option 2", "C) Option 3", "D) Option 4"],
    "correct": 0,
    "explanation": "Why option A is correct."
  }}
]
Note: 'correct' is the zero-based index of the correct option in the options array (0, 1, 2, or 3)."""

        elif tool_type == "mind_map":
            prompt_text = f"""Generate a hierarchical Mind Map of key concepts based on the following source material.

Title: {title or 'Concept Mind Map'}
Content:
{content_excerpt}

Return a JSON object with key "mind_map" which is a nested tree object:
{{
  "label": "Central Subject",
  "children": [
    {{
      "label": "Main Topic 1",
      "children": [
        {{"label": "Sub-concept 1.1"}},
        {{"label": "Sub-concept 1.2"}}
      ]
    }},
    {{
      "label": "Main Topic 2",
      "children": [
        {{"label": "Sub-concept 2.1"}}
      ]
    }}
  ]
}}"""

        elif tool_type == "ai_report":
            prompt_text = f"""Generate an in-depth AI Study Report based on the following source material.

Title: {title or 'Deep-Dive AI Report'}
Content:
{content_excerpt}

Return a JSON object with key "report" containing long-form markdown formatted text:
Include an Executive Summary, Section-by-Section Deep Dives, Comparative Analysis, and Strategic Recommendations."""

        else:
            raise ValueError(f"Unsupported tool type: {tool_type}")

        prompt = Prompt(
            text=prompt_text,
            system_prompt="You are an expert AI study tool generator. Return valid JSON ONLY without any markdown code fences.",
            temperature=0.3,
            max_tokens=2000,
        )

        response = await self.provider.generate(prompt)
        raw = response.text.strip()

        # Clean code fences if returned
        if raw.startswith("```"):
            raw = raw.split("\n", 1)[-1]
        if raw.endswith("```"):
            raw = raw.rsplit("```", 1)[0]
        raw = raw.strip()

        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            logger.warning(f"Failed to parse JSON for {tool_type}, returning wrapped text")
            if tool_type == "summary":
                return {"summary": raw}
            elif tool_type == "key_takeaways":
                return {"takeaways": raw}
            elif tool_type == "ai_report":
                return {"report": raw}
            else:
                return {"raw": raw}
