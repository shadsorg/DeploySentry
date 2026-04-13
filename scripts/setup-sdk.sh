#!/bin/sh
set -e

# DeploySentry SDK Setup Script
# Detects project language, installs the appropriate SDK, and writes an LLM
# integration prompt into CLAUDE.md

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

DS_API_KEY=""
DS_ENVIRONMENT="production"
DS_PROJECT=""
DS_BASE_URL="https://api.deploysentry.io"

usage() {
  echo "Usage: $0 --api-key <key> [--environment <env>] [--project <slug>] [--base-url <url>]" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --api-key)
      DS_API_KEY="$2"
      shift 2
      ;;
    --environment)
      DS_ENVIRONMENT="$2"
      shift 2
      ;;
    --project)
      DS_PROJECT="$2"
      shift 2
      ;;
    --base-url)
      DS_BASE_URL="$2"
      shift 2
      ;;
    --help|-h)
      usage
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [ -z "$DS_API_KEY" ]; then
  echo "Error: --api-key is required" >&2
  usage
fi

if [ -z "$DS_PROJECT" ]; then
  printf "Enter project slug: "
  read -r DS_PROJECT
  if [ -z "$DS_PROJECT" ]; then
    echo "Error: project slug is required" >&2
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Language detection
# ---------------------------------------------------------------------------

detect_language() {
  # Flutter — pubspec.yaml
  if [ -f "pubspec.yaml" ]; then
    echo "flutter"
    return
  fi

  # Java/Kotlin — pom.xml or build.gradle
  if [ -f "pom.xml" ] || [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then
    echo "java"
    return
  fi

  # Ruby — Gemfile
  if [ -f "Gemfile" ]; then
    echo "ruby"
    return
  fi

  # Go — go.mod
  if [ -f "go.mod" ]; then
    echo "go"
    return
  fi

  # Python — requirements.txt, pyproject.toml, or setup.py
  if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ] || [ -f "setup.py" ]; then
    echo "python"
    return
  fi

  # Node / React — package.json
  if [ -f "package.json" ]; then
    # Check for react or next in dependencies
    if grep -qE '"(react|next)"' package.json 2>/dev/null; then
      echo "react"
    else
      echo "node"
    fi
    return
  fi

  echo ""
}

LANG_DETECTED=$(detect_language)

if [ -z "$LANG_DETECTED" ]; then
  echo "Error: Could not detect project language." >&2
  echo "Supported languages: node, react, go, python, java, ruby, flutter" >&2
  echo "Make sure you are running this script from your project root directory." >&2
  exit 1
fi

echo "Detected language: $LANG_DETECTED"

# ---------------------------------------------------------------------------
# SDK installation
# ---------------------------------------------------------------------------

install_sdk() {
  case "$LANG_DETECTED" in
    node)
      echo "Installing @deploysentry/node..."
      npm install @deploysentry/node
      ;;
    react)
      echo "Installing @deploysentry/react..."
      npm install @deploysentry/react
      ;;
    go)
      echo "Installing deploysentry-go..."
      go get github.com/shadsorg/deploysentry-go
      ;;
    python)
      echo "Installing deploysentry (pip)..."
      pip install deploysentry
      if [ -f "requirements.txt" ]; then
        if ! grep -q "^deploysentry" requirements.txt 2>/dev/null; then
          echo "deploysentry" >> requirements.txt
          echo "Added deploysentry to requirements.txt"
        fi
      fi
      ;;
    java)
      echo ""
      echo "Java/Kotlin detected. Add the following dependency to your build file:"
      echo ""
      echo "Maven (pom.xml):"
      echo "  <dependency>"
      echo "    <groupId>io.deploysentry</groupId>"
      echo "    <artifactId>deploysentry-java</artifactId>"
      echo "    <version>LATEST</version>"
      echo "  </dependency>"
      echo ""
      echo "Gradle (build.gradle):"
      echo "  implementation 'io.deploysentry:deploysentry-java:LATEST'"
      echo ""
      ;;
    ruby)
      if [ -f "Gemfile" ]; then
        if ! grep -q "deploysentry" Gemfile 2>/dev/null; then
          echo "gem 'deploysentry'" >> Gemfile
          echo "Added deploysentry gem to Gemfile"
        fi
      fi
      echo "Running bundle install..."
      bundle install
      ;;
    flutter)
      echo "Adding deploysentry Flutter package..."
      flutter pub add deploysentry
      ;;
  esac
}

install_sdk
