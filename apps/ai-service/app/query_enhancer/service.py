import json
from app.providers import LLMProvider, Prompt


class QueryEnhancementResult:
    """Result of the 3-in-1 query understanding call."""

    def __init__(self, step_back: str, rewritten: str, sub_queries: list[str]):
        self.step_back = step_back
        self.rewritten = rewritten
        self.sub_queries = sub_queries


class QueryEnhancerService:
    """Implements query understanding from the advanced RAG pipeline:
    1. Step-back prompting — broader background question
    2. Query rewriting — fix typos, make explicit and self-contained
    3. Sub-query decomposition — break into 3 focused sub-questions
    """

    def __init__(self, provider: LLMProvider):
        self.provider = provider

    async def enhance(self, query: str, prompt_version: str = "1.0") -> QueryEnhancementResult:
        system_prompt = (
            "You are an expert query analyzer for a RAG (Retrieval Augmented Generation) system. "
            "Your job is to transform a user's raw question into multiple query variants that "
            "maximize retrieval quality from a vector database of course material.\n\n"
            "You MUST return valid JSON with exactly this structure:\n"
            "{\n"
            '  "step_back": "A broader, more general background question that provides '
            'foundational context for answering the original query",\n'
            '  "rewritten": "The original query cleaned up — fix typos, resolve pronouns, '
            'make it explicit and self-contained without changing the intent",\n'
            '  "sub_queries": [\n'
            '    "First focused sub-question that addresses one specific aspect",\n'
            '    "Second focused sub-question covering another angle",\n'
            '    "Third focused sub-question exploring a related dimension"\n'
            "  ]\n"
            "}"
        )

        prompt = Prompt(
            text=f"User query: {query}",
            system_prompt=system_prompt,
            temperature=0.3,
            max_tokens=500,
            prompt_version=prompt_version,
        )

        response = await self.provider.generate(prompt)
        parsed = json.loads(response.text)

        return QueryEnhancementResult(
            step_back=parsed["step_back"],
            rewritten=parsed["rewritten"],
            sub_queries=parsed["sub_queries"][:3],  # cap at 3
        )
