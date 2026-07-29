from app.providers import LLMProvider
from app.providers.base import Prompt, Response


class SummaryGeneratorService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider

    async def generate_summary(self, content: str, prompt_version: str = "1.0") -> str:
        """Generate a real NotebookLM-style Source Guide summary."""
        prompt = Prompt(
            text=f"""Analyze this source content and generate a comprehensive NotebookLM Source Guide summary.
Describe the core focus, key technical concepts, tools covered, and practical takeaways.

Content Excerpt:
{content[:6000]}

Return a clean, professional 2-paragraph Source Guide.""",
            system_prompt="You are an expert technical assistant that writes comprehensive NotebookLM Source Guides.",
            temperature=0.3,
            max_tokens=300,
            prompt_version=prompt_version,
        )

        response = await self.provider.generate(prompt)
        return response.text.strip()