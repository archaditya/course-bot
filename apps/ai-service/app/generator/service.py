from typing import AsyncGenerator
from app.providers import LLMProvider, Token, Prompt


class GeneratorService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider
    
    async def generate(
        self,
        query: str,
        context: str,
        prompt_version: str = "1.0",
    ) -> AsyncGenerator[Token, None]:
        """Generate response with streaming."""
        system_prompt = """You are a helpful AI assistant that answers questions based on the provided course content.
Always ground your answers in the given context. If the answer is not in the context, say so clearly.
Include citations in the format [chunk_id] when referencing specific content."""
        
        prompt = Prompt(
            text=f"""Context:
{context}

Question: {query}

Answer:""",
            system_prompt=system_prompt,
            temperature=0.7,
            max_tokens=2000,
            prompt_version=prompt_version,
        )
        
        async for token in self.provider.stream(prompt):
            yield token