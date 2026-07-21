from app.providers import LLMProvider, Prompt, Response
from config.settings import settings


class EvaluatorService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider
    
    async def evaluate(
        self,
        query: str,
        response: str,
        context: str,
        prompt_version: str = "1.0",
    ) -> float:
        """Evaluate response quality on scale 1-10."""
        prompt = Prompt(
            text=f"""Evaluate this Q&A pair on a scale of 1-10 based on:
1. Accuracy (is the answer correct based on context?)
2. Relevance (does it answer the question?)
3. Completeness (does it provide sufficient detail?)
4. Grounding (is it grounded in the provided context?)

Query: {query}
Response: {response}
Context: {context[:2000]}

Return only a single number (1-10).""",
            system_prompt="You are an evaluator that scores AI responses.",
            temperature=0.1,
            max_tokens=10,
            prompt_version=prompt_version,
        )
        
        result = await self.provider.generate(prompt)
        try:
            score = float(result.text.strip())
            return max(1.0, min(10.0, score))
        except ValueError:
            return 5.0  # Default score if parsing fails
    
    def passes_threshold(self, score: float) -> bool:
        """Check if score meets threshold."""
        return score >= settings.evaluator_threshold