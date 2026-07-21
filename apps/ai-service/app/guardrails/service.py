from app.guardrails.pii import PIIChecker
from app.guardrails.injection import InjectionChecker
from app.providers import GuardrailProvider


class GuardrailsService:
    def __init__(self, provider: GuardrailProvider):
        self.pii_checker = PIIChecker(provider)
        self.injection_checker = InjectionChecker(provider)
    
    async def check_query(self, query: str) -> tuple[bool, str]:
        """Check query against all guardrails."""
        # Check PII
        pii_result = await self.pii_checker.check(query)
        if pii_result.has_pii:
            return False, f"PII detected: {', '.join(pii_result.detected_types)}"
        
        # Check injection
        injection_result = await self.injection_checker.check(query)
        if injection_result.is_injection:
            return False, f"Prompt injection detected: {injection_result.detected_pattern}"
        
        return True, "Passed"
    
    async def check_output(self, output: str) -> tuple[bool, str]:
        """Check output against guardrails."""
        pii_result = await self.pii_checker.check(output)
        if pii_result.has_pii:
            return False, f"PII in output: {', '.join(pii_result.detected_types)}"
        
        return True, "Passed"