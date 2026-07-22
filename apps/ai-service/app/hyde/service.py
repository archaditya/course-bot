from app.providers import LLMProvider, Prompt


class HydeService:
    """HyDE (Hypothetical Document Embeddings) — generates a concise, factual
    passage that directly answers the user's question as if it were an excerpt
    from a real document.

    The hypothetical document is then embedded by the caller for vector search.
    Its embedding is often closer to relevant real documents than the bare
    question embedding, because it shares vocabulary and structure with actual
    course content.
    """

    def __init__(self, provider: LLMProvider):
        self.provider = provider

    async def generate(self, query: str, prompt_version: str = "1.0") -> str:
        system_prompt = (
            "You are a knowledgeable instructor writing course material. "
            "Given a student's question, write a concise passage (3-5 sentences) "
            "that directly answers the question as if it were an excerpt from "
            "a real textbook or lecture transcript.\n\n"
            "Rules:\n"
            "- Write in a factual, instructional tone\n"
            "- Use specific technical terms relevant to the topic\n"
            "- Do NOT say 'the student asked' or reference the question itself\n"
            "- Write as if this passage already exists in the course material\n"
            "- Keep it between 3-5 sentences, no more"
        )

        prompt = Prompt(
            text=f"Student question: {query}",
            system_prompt=system_prompt,
            temperature=0.5,
            max_tokens=300,
            prompt_version=prompt_version,
        )

        response = await self.provider.generate(prompt)
        return response.text.strip()
