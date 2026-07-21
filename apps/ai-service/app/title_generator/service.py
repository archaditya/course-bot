from app.providers import LLMProvider
from app.providers.base import Prompt, Response


class TitleGeneratorService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider
    
    async def generate_title(self, content: str, prompt_version: str = "1.0") -> str:
        """Generate a short, descriptive title for content."""
        prompt = Prompt(
            text=f"""Generate a concise, descriptive title (max 10 words) for this content:

{content[:2000]}

Return only the title, no explanation.""",
            system_prompt="You are a helpful assistant that generates clear, concise titles.",
            temperature=0.5,
            max_tokens=50,
            prompt_version=prompt_version,
        )
        
        response = await self.provider.generate(prompt)
        return response.text.strip()