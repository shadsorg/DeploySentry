import React, { useState } from 'react';
import { useParams } from 'react-router-dom';

// ---------------------------------------------------------------------------
// SDK data
// ---------------------------------------------------------------------------

interface SDK {
  name: string;
  iconLetter: string;
  iconColor: string;
  packageName: string;
  installCmd: string;
  version: string;
  description: string;
}

const SDKS: SDK[] = [
  {
    name: 'Go',
    iconLetter: 'G',
    iconColor: '#00ADD8',
    packageName: 'github.com/deploysentry/deploysentry-go',
    installCmd: 'go get github.com/deploysentry/deploysentry-go',
    version: 'v1.0.0',
    description:
      'Evaluate feature flags in Go services with a lightweight, thread-safe client. Supports context-based targeting and real-time flag updates.',
  },
  {
    name: 'Node.js / TypeScript',
    iconLetter: 'N',
    iconColor: '#339933',
    packageName: '@deploysentry/sdk',
    installCmd: 'npm install @deploysentry/sdk',
    version: '1.0.0',
    description:
      'First-class TypeScript support for server-side and edge runtime flag evaluation. Works with Express, Fastify, Next.js API routes, and more.',
  },
  {
    name: 'Python',
    iconLetter: 'P',
    iconColor: '#3776AB',
    packageName: 'deploysentry',
    installCmd: 'pip install deploysentry',
    version: '1.0.0',
    description:
      'Integrate feature flags into Django, Flask, or FastAPI applications. Supports async evaluation and local flag caching.',
  },
  {
    name: 'Java',
    iconLetter: 'J',
    iconColor: '#ED8B00',
    packageName: 'io.deploysentry:deploysentry-java',
    installCmd:
      '<dependency>\n  <groupId>io.deploysentry</groupId>\n  <artifactId>deploysentry-java</artifactId>\n  <version>1.0.0</version>\n</dependency>',
    version: '1.0.0',
    description:
      'Enterprise-grade SDK for Spring Boot and JVM applications. Includes annotation-based flag evaluation and Spring auto-configuration.',
  },
  {
    name: 'React',
    iconLetter: 'R',
    iconColor: '#61DAFB',
    packageName: '@deploysentry/react',
    installCmd: 'npm install @deploysentry/react',
    version: '1.0.0',
    description:
      'React hooks and components for client-side feature flags. Includes <FeatureGate> component and useFlag / useFlags hooks.',
  },
  {
    name: 'Flutter / Dart',
    iconLetter: 'F',
    iconColor: '#02569B',
    packageName: 'deploysentry_flutter',
    installCmd: 'flutter pub add deploysentry_flutter',
    version: '1.0.0',
    description:
      'Native Flutter plugin for iOS and Android feature flag evaluation. Supports offline mode with local flag caching.',
  },
  {
    name: 'Ruby',
    iconLetter: 'R',
    iconColor: '#CC342D',
    packageName: 'deploysentry',
    installCmd: 'gem install deploysentry',
    version: '1.0.0',
    description:
      'Ruby gem for Rails and Sinatra applications. Includes Rack middleware for request-scoped flag evaluation.',
  },
];

// ---------------------------------------------------------------------------
// Quick Start code samples
// ---------------------------------------------------------------------------

type Language = 'go' | 'typescript' | 'python' | 'java' | 'ruby' | 'dart';

const CODE_SAMPLES: Record<Language, { label: string; code: string }> = {
  go: {
    label: 'Go',
    code: `package main

import (
    "context"
    "fmt"
    ds "github.com/deploysentry/deploysentry-go"
)

func main() {
    // 1. Initialize the client
    client, err := ds.NewClient(ds.Config{
        APIKey:      "ds_prod_abc123",
        Environment: "production",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 2. Evaluate a flag with context
    ctx := context.Background()
    result := client.BoolFlag(ctx, "new-checkout-flow", ds.EvalContext{
        UserID:     "user-42",
        Attributes: map[string]interface{}{"plan": "enterprise"},
    })

    if result.Value {
        fmt.Println("New checkout flow is enabled!")
    }

    // 3. Access flag metadata
    fmt.Printf("Category: %s\\n", result.Flag.Category)   // "release"
    fmt.Printf("Owners:   %v\\n", result.Flag.Owners)      // ["team-checkout"]
    fmt.Printf("Purpose:  %s\\n", result.Flag.Purpose)     // "Rollout of redesigned checkout"
}`,
  },
  typescript: {
    label: 'TypeScript',
    code: `import { DeploySentry } from "@deploysentry/sdk";

// 1. Initialize the client
const client = new DeploySentry({
  apiKey: "ds_prod_abc123",
  environment: "production",
});

// 2. Evaluate a flag with context
const result = await client.boolFlag("new-checkout-flow", {
  userId: "user-42",
  attributes: { plan: "enterprise" },
});

if (result.value) {
  console.log("New checkout flow is enabled!");
}

// 3. Access flag metadata
console.log("Category:", result.flag.category);   // "release"
console.log("Owners:",   result.flag.owners);      // ["team-checkout"]
console.log("Purpose:",  result.flag.purpose);     // "Rollout of redesigned checkout"`,
  },
  python: {
    label: 'Python',
    code: `from deploysentry import DeploySentryClient

# 1. Initialize the client
client = DeploySentryClient(
    api_key="ds_prod_abc123",
    environment="production",
)

# 2. Evaluate a flag with context
result = client.bool_flag("new-checkout-flow", context={
    "user_id": "user-42",
    "attributes": {"plan": "enterprise"},
})

if result.value:
    print("New checkout flow is enabled!")

# 3. Access flag metadata
print(f"Category: {result.flag.category}")   # "release"
print(f"Owners:   {result.flag.owners}")      # ["team-checkout"]
print(f"Purpose:  {result.flag.purpose}")     # "Rollout of redesigned checkout"`,
  },
  java: {
    label: 'Java',
    code: `import io.deploysentry.DeploySentryClient;
import io.deploysentry.EvalContext;
import io.deploysentry.FlagResult;

public class Example {
    public static void main(String[] args) {
        // 1. Initialize the client
        DeploySentryClient client = DeploySentryClient.builder()
            .apiKey("ds_prod_abc123")
            .environment("production")
            .build();

        // 2. Evaluate a flag with context
        EvalContext ctx = EvalContext.builder()
            .userId("user-42")
            .attribute("plan", "enterprise")
            .build();

        FlagResult<Boolean> result = client.boolFlag("new-checkout-flow", ctx);

        if (result.getValue()) {
            System.out.println("New checkout flow is enabled!");
        }

        // 3. Access flag metadata
        System.out.println("Category: " + result.getFlag().getCategory());
        System.out.println("Owners:   " + result.getFlag().getOwners());
        System.out.println("Purpose:  " + result.getFlag().getPurpose());
    }
}`,
  },
  ruby: {
    label: 'Ruby',
    code: `require "deploysentry"

# 1. Initialize the client
client = DeploySentry::Client.new(
  api_key: "ds_prod_abc123",
  environment: "production"
)

# 2. Evaluate a flag with context
result = client.bool_flag("new-checkout-flow", context: {
  user_id: "user-42",
  attributes: { plan: "enterprise" }
})

if result.value
  puts "New checkout flow is enabled!"
end

# 3. Access flag metadata
puts "Category: #{result.flag.category}"   # "release"
puts "Owners:   #{result.flag.owners}"      # ["team-checkout"]
puts "Purpose:  #{result.flag.purpose}"     # "Rollout of redesigned checkout"`,
  },
  dart: {
    label: 'Dart / Flutter',
    code: `import 'package:deploysentry_flutter/deploysentry_flutter.dart';

Future<void> main() async {
  // 1. Initialize the client
  final client = DeploySentryClient(
    apiKey: 'ds_prod_abc123',
    environment: 'production',
  );
  await client.initialize();

  // 2. Evaluate a flag with context
  final result = await client.boolFlag('new-checkout-flow', context: EvalContext(
    userId: 'user-42',
    attributes: {'plan': 'enterprise'},
  ));

  if (result.value) {
    print('New checkout flow is enabled!');
  }

  // 3. Access flag metadata
  print('Category: \${result.flag.category}');   // "release"
  print('Owners:   \${result.flag.owners}');      // ["team-checkout"]
  print('Purpose:  \${result.flag.purpose}');     // "Rollout of redesigned checkout"
}`,
  },
};

const LANGUAGES: Language[] = ['go', 'typescript', 'python', 'java', 'ruby', 'dart'];

// ---------------------------------------------------------------------------
// Flag categories reference
// ---------------------------------------------------------------------------

const FLAG_CATEGORIES = [
  {
    key: 'release',
    label: 'Release',
    description:
      'Gates tied to a specific software release. Typically short-lived and removed once the release is fully rolled out.',
  },
  {
    key: 'feature',
    label: 'Feature',
    description:
      'Controls access to product features. May be long-lived for premium features or short-lived for gradual rollouts.',
  },
  {
    key: 'experiment',
    label: 'Experiment',
    description:
      'Used for A/B tests and multivariate experiments. Should have a defined end date and success metrics.',
  },
  {
    key: 'ops',
    label: 'Ops',
    description:
      'Operational flags for circuit breakers, maintenance modes, and infrastructure controls. Often long-lived.',
  },
  {
    key: 'permission',
    label: 'Permission',
    description:
      'Controls user-level or role-level permissions. Typically long-lived and tied to entitlements or access policies.',
  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const SDKsPage: React.FC = () => {
  const { projectSlug } = useParams();
  const projectName = projectSlug ?? '';

  const [activeLanguage, setActiveLanguage] = useState<Language>('go');

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <h1>{projectName ? `${projectName} — SDKs & Docs` : 'SDKs & Integration'}</h1>
        <p>Install a DeploySentry SDK to evaluate feature flags in your application</p>
      </div>

      {/* SDK cards grid */}
      <div className="sdk-grid mb-4">
        {SDKS.map((sdk) => (
          <div key={sdk.name} className="sdk-card">
            <div className="sdk-card-header">
              <div
                className="sdk-card-icon"
                style={{ background: sdk.iconColor, color: '#fff' }}
              >
                {sdk.iconLetter}
              </div>
              <div>
                <div className="sdk-card-title">{sdk.name}</div>
                <div className="sdk-card-version">{sdk.version}</div>
              </div>
            </div>
            <div className="sdk-card-desc">{sdk.description}</div>
            <div className="sdk-install-cmd">
              <code>{sdk.installCmd.includes('\n') ? sdk.installCmd.split('\n')[0] + ' ...' : sdk.installCmd}</code>
              <button
                className="btn-icon"
                title="Copy install command"
                onClick={() => handleCopy(sdk.installCmd)}
              >
                📋
              </button>
            </div>
            <a href="#quick-start" className="btn btn-secondary btn-sm">
              View Documentation
            </a>
          </div>
        ))}
      </div>

      {/* Quick Start Guide */}
      <div className="card mb-4" id="quick-start">
        <div className="card-header">
          <span className="card-title">Quick Start Guide</span>
        </div>
        <p className="text-secondary text-sm mb-4">
          Every DeploySentry SDK follows the same three-step pattern: initialize the client,
          evaluate a flag with context, and access flag metadata.
        </p>

        {/* Language tabs */}
        <div className="tabs">
          {LANGUAGES.map((lang) => (
            <button
              key={lang}
              className={`tab ${activeLanguage === lang ? 'active' : ''}`}
              onClick={() => setActiveLanguage(lang)}
            >
              {CODE_SAMPLES[lang].label}
            </button>
          ))}
        </div>

        {/* Code block */}
        <div className="code-block">
          <div className="code-header">
            <span className="code-lang">{CODE_SAMPLES[activeLanguage].label}</span>
            <button
              className="btn-icon"
              title="Copy code"
              onClick={() => handleCopy(CODE_SAMPLES[activeLanguage].code)}
            >
              📋
            </button>
          </div>
          <pre>
            <code>{CODE_SAMPLES[activeLanguage].code}</code>
          </pre>
        </div>
      </div>

      {/* Flag Categories */}
      <div className="card mb-4">
        <div className="card-header">
          <span className="card-title">Flag Categories</span>
        </div>
        <p className="text-secondary text-sm mb-4">
          Every flag in DeploySentry is assigned a category. Categories help teams understand the
          purpose and expected lifecycle of each flag.
        </p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {FLAG_CATEGORIES.map((cat) => (
            <div key={cat.key} className="flex items-center gap-3">
              <span
                className={`badge badge-${cat.key}`}
                style={{ minWidth: 100, justifyContent: 'center' }}
              >
                {cat.label}
              </span>
              <span className="text-sm text-secondary">{cat.description}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Getting an API Key */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">Getting an API Key</span>
        </div>
        <p className="text-secondary text-sm mb-2">
          To authenticate your SDK, you need an API key with the appropriate scopes:
        </p>
        <ol style={{ paddingLeft: 20, color: 'var(--color-text-secondary)', fontSize: 14, lineHeight: 1.8 }}>
          <li>
            Navigate to <a href="/settings">Settings</a> and select the <strong>API Keys</strong> tab.
          </li>
          <li>
            Click <strong>Create API Key</strong> and give it a descriptive name (e.g., &quot;Production
            Backend&quot;).
          </li>
          <li>
            Select the <code className="font-mono">flags:read</code> scope for flag evaluation. Add{' '}
            <code className="font-mono">deploys:read</code> if your service also needs deployment
            information.
          </li>
          <li>
            Copy the generated key immediately &mdash; it will only be shown once.
          </li>
        </ol>
      </div>
    </div>
  );
};

export default SDKsPage;
