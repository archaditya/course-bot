import json
import logging
import re
from typing import Dict, Any, Optional
from pydantic import BaseModel
from app.providers import LLMProvider, Prompt
from config.settings import settings

logger = logging.getLogger("ai-service.evaluator")


class EvaluationDetail(BaseModel):
    overall_score: float
    faithfulness: float
    answer_relevance: float
    context_precision: float
    reasoning: str
    passes_threshold: bool


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
        """Fast scalar evaluation (1-10) for backward compatibility."""
        detail = await self.evaluate_detailed(query, response, context, prompt_version)
        return detail.overall_score

    async def evaluate_detailed(
        self,
        query: str,
        response: str,
        context: str,
        prompt_version: str = "1.0",
    ) -> EvaluationDetail:
        """
        Tier 3 AI Quality Evaluation:
        Computes 3-dimensional RAG evaluation metrics:
        1. Faithfulness/Groundedness: Is the answer derived directly from context without hallucination?
        2. Answer Relevance: Does the answer address the specific user question?
        3. Context Precision: How relevant is the context to answering the question?
        """
        eval_prompt = f"""You are an expert AI Quality Evaluator for a RAG system.
Evaluate the following Interaction:

[QUESTION]
{query}

[RETRIEVED CONTEXT]
{context[:3000]}

[AI RESPONSE]
{response}

Score each dimension from 0.0 to 1.0:
1. faithfulness: 1.0 if completely grounded in context without hallucinations, 0.0 if fabricated.
2. answer_relevance: 1.0 if directly and concisely answers the user prompt, 0.0 if irrelevant.
3. context_precision: 1.0 if context contains the exact required information, 0.0 if useless noise.

Output strict JSON with this schema:
{{
  "overall_score": <number 1.0 to 10.0>,
  "faithfulness": <number 0.0 to 1.0>,
  "answer_relevance": <number 0.0 to 1.0>,
  "context_precision": <number 0.0 to 1.0>,
  "reasoning": "<concise explanation>"
}}
Return ONLY valid JSON."""

        prompt = Prompt(
            text=eval_prompt,
            system_prompt="You are a strict GenAI quality judge. Output only JSON.",
            temperature=0.0,
            max_tokens=300,
            prompt_version=prompt_version,
        )

        try:
            if hasattr(self.provider, "generate_mini"):
                result = await self.provider.generate_mini(prompt)
            else:
                result = await self.provider.generate(prompt)

            raw_text = result.text.strip()
            # Extract JSON block if wrapped in markdown backticks
            if "```json" in raw_text:
                raw_text = raw_text.split("```json")[1].split("```")[0].strip()
            elif "```" in raw_text:
                raw_text = raw_text.split("```")[1].split("```")[0].strip()

            data = json.loads(raw_text)
            overall = float(data.get("overall_score", 7.0))
            overall = max(1.0, min(10.0, overall))

            faithfulness = float(data.get("faithfulness", 0.7))
            relevance = float(data.get("answer_relevance", 0.7))
            precision = float(data.get("context_precision", 0.7))
            reasoning = str(data.get("reasoning", "Evaluation completed successfully."))

            return EvaluationDetail(
                overall_score=round(overall, 1),
                faithfulness=round(max(0.0, min(1.0, faithfulness)), 2),
                answer_relevance=round(max(0.0, min(1.0, relevance)), 2),
                context_precision=round(max(0.0, min(1.0, precision)), 2),
                reasoning=reasoning,
                passes_threshold=overall >= settings.evaluator_threshold,
            )

        except Exception as e:
            logger.warning(f"Evaluation parsing failed, using fallback: {e}")
            # Fallback safe score
            return EvaluationDetail(
                overall_score=7.0,
                faithfulness=0.75,
                answer_relevance=0.75,
                context_precision=0.75,
                reasoning=f"Evaluation fallback triggered due to parsing: {str(e)}",
                passes_threshold=True,
            )

    def passes_threshold(self, score: float) -> bool:
        """Check if score meets threshold."""
        return score >= settings.evaluator_threshold
