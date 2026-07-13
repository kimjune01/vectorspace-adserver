from .client import VectorSpace
from .prompt import INTENT_PROMPT

# Historical alias; tests and older integrations use the Client suffix.
VectorSpaceClient = VectorSpace

__all__ = ["VectorSpace", "VectorSpaceClient", "INTENT_PROMPT"]
