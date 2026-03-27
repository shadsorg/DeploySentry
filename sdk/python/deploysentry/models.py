"""Data models for the DeploySentry Python SDK."""

from __future__ import annotations

import enum
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Dict, List, Optional


class FlagCategory(enum.Enum):
    """Categories for feature flags."""

    RELEASE = "release"
    FEATURE = "feature"
    EXPERIMENT = "experiment"
    OPS = "ops"
    PERMISSION = "permission"


@dataclass
class FlagMetadata:
    """Rich metadata associated with a feature flag."""

    category: FlagCategory = FlagCategory.FEATURE
    purpose: str = ""
    owners: List[str] = field(default_factory=list)
    is_permanent: bool = False
    expires_at: Optional[datetime] = None
    tags: List[str] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> FlagMetadata:
        """Construct FlagMetadata from an API response dictionary."""
        category = FlagCategory.FEATURE
        raw_category = data.get("category")
        if raw_category:
            try:
                category = FlagCategory(raw_category)
            except ValueError:
                pass

        expires_at = None
        raw_expires = data.get("expires_at")
        if raw_expires:
            try:
                expires_at = datetime.fromisoformat(raw_expires.replace("Z", "+00:00"))
            except (ValueError, AttributeError):
                pass

        return cls(
            category=category,
            purpose=data.get("purpose", ""),
            owners=data.get("owners", []),
            is_permanent=data.get("is_permanent", False),
            expires_at=expires_at,
            tags=data.get("tags", []),
        )


@dataclass
class Flag:
    """Representation of a feature flag."""

    key: str
    enabled: bool = False
    value: Any = None
    metadata: FlagMetadata = field(default_factory=FlagMetadata)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> Flag:
        """Construct a Flag from an API response dictionary."""
        metadata_raw = data.get("metadata", {})
        metadata = FlagMetadata.from_dict(metadata_raw) if metadata_raw else FlagMetadata()
        return cls(
            key=data.get("key", ""),
            enabled=data.get("enabled", False),
            value=data.get("value"),
            metadata=metadata,
        )


@dataclass
class EvaluationContext:
    """Context provided when evaluating a flag."""

    user_id: Optional[str] = None
    org_id: Optional[str] = None
    attributes: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Serialise to a dictionary for the API request."""
        result: Dict[str, Any] = {}
        if self.user_id is not None:
            result["user_id"] = self.user_id
        if self.org_id is not None:
            result["org_id"] = self.org_id
        if self.attributes is not None:
            result["attributes"] = self.attributes
        return result


@dataclass
class EvaluationResult:
    """Full result of a flag evaluation, including metadata."""

    key: str
    enabled: bool
    value: Any
    reason: str = ""
    variant: Optional[str] = None
    metadata: FlagMetadata = field(default_factory=FlagMetadata)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> EvaluationResult:
        """Construct an EvaluationResult from an API response dictionary."""
        metadata_raw = data.get("metadata", {})
        metadata = FlagMetadata.from_dict(metadata_raw) if metadata_raw else FlagMetadata()
        return cls(
            key=data.get("key", ""),
            enabled=data.get("enabled", False),
            value=data.get("value"),
            reason=data.get("reason", ""),
            variant=data.get("variant"),
            metadata=metadata,
        )
