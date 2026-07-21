from app.providers import LLMProvider
from app.providers.base import Prompt, Response


class SummaryGeneratorService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider
    
    async def generate_summary(self, content: str, prompt_version: str = "1.0") -> str:
        """Generate a 1-2 sentence summary of content."""
        prompt = Prompt(
            text=f"""Generate a 1-2 sentence summary of this content:

{content[:3000]}

Return only the summary, no explanation.""",
            system_prompt="You are a helpful assistant that writes clear, concise summaries.",
            temperature=0.5,
            max_tokens=100,
            prompt_version=prompt_version,
        )
        
        response = await self.provider.generate(prompt)
        return response.text.strip()