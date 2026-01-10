import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Zeus TextMate grammar for syntax highlighting
const zeusGrammar = {
  name: 'zeus',
  scopeName: 'source.zeus',
  patterns: [
    {
      name: 'keyword.control.zeus',
      match: '\\b(if|else|return|while|new|class|private|protected|public|this|try|throw|catch|finally|constructor)\\b'
    },
    {
      name: 'storage.type.zeus',
      match: '\\b(type|function|extern)\\b'
    },
    {
      name: 'storage.modifier.zeus',
      match: '\\b(let|const)\\b'
    },
    {
      name: 'constant.numeric.zeus',
      match: '\\b\\d+(\\.\\d+)?\\b'
    },
    {
      name: 'constant.boolean.zeus',
      match: '\\b(true|false)\\b'
    },
    {
      name: 'string.quoted.double.zeus',
      begin: '"',
      end: '"',
      patterns: [
        {
          name: 'constant.character.escape.zeus',
          match: '\\\\.'
        }
      ]
    },
    {
      match: '\\b([a-zA-Z_][a-zA-Z0-9_]*)\\s*(?=\\()',
      captures: {
        1: {
          name: 'entity.name.function.zeus'
        }
      }
    },
    {
      name: 'comment.line.double-slash.zeus',
      match: '//.*$'
    },
    {
      name: 'support.type.zeus',
      match: '\\b(i8|i16|i32|i64|i128|u8|u16|u32|u64|f16|f32|f64|f128|boolean|void|string|null)\\b'
    },
    {
      name: 'keyword.control.import.zeus',
      match: '\\b(import)\\b'
    },
    {
      name: 'keyword.control.from.zeus',
      match: '\\b(from)\\b'
    },
    {
      name: 'keyword.control.export.zeus',
      match: '\\b(export)\\b'
    },
    {
      match: "import\\s*({[^}]*})\\s*from\\s*(['\"][^'\"]*['\"])",
      captures: {
        1: {
          name: 'variable.other.zeus'
        },
        2: {
          name: 'string.quoted.single.zeus'
        }
      }
    },
    {
      match: '\\b[a-zA-Z_][a-zA-Z0-9_]*\\b',
      name: 'variable.other.zeus'
    }
  ]
};

export default defineConfig({
  integrations: [
    starlight({
      title: 'Zeus',
      description: 'A modern, garbage-collected programming language with TypeScript-inspired syntax and native performance.',
      logo: {
        light: './src/assets/zeus-logo-light.svg',
        dark: './src/assets/zeus-logo-dark.svg',
        replacesTitle: false,
      },
      social: {
        github: 'https://github.com/ameerthehacker/zeus',
      },
      customCss: [
        './src/styles/custom.css',
      ],
      expressiveCode: {
        themes: ['github-dark', 'github-light'],
        shiki: {
          langs: [zeusGrammar],
        },
      },
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Why Zeus?', slug: 'why-zeus' },
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Hello World', slug: 'getting-started/hello-world' },
            { label: 'IDE Setup', slug: 'getting-started/ide-setup' },
          ],
        },
        {
          label: 'Language Guide',
          items: [
            { label: 'Variables', slug: 'language/variables' },
            { label: 'Types', slug: 'language/types' },
            { label: 'Functions', slug: 'language/functions' },
            { label: 'Built-in Functions', slug: 'language/built-in-functions' },
            { label: 'Classes', slug: 'language/classes' },
            { label: 'Arrays', slug: 'language/arrays' },
            { label: 'Strings', slug: 'language/strings' },
            { label: 'Control Flow', slug: 'language/control-flow' },
            { label: 'Operators', slug: 'language/operators' },
            { label: 'Modules', slug: 'language/modules' },
          ],
        },
        {
          label: 'Tooling',
          items: [
            { label: 'Language Server', slug: 'tooling/lsp' },
          ],
        },
        {
          label: 'Examples',
          items: [
            { label: 'Fibonacci', slug: 'examples/fibonacci' },
            { label: 'Calculator', slug: 'examples/calculator' },
          ],
        },
        {
          label: 'Developer',
          items: [
            { label: 'Building from Source', slug: 'developer/building-from-source' },
            { label: 'Compiler Architecture', slug: 'developer/compiler-architecture' },
          ],
        },
        {
          label: 'Coming Soon',
          badge: { text: 'Planned', variant: 'note' },
          items: [
            { label: 'Inheritance', slug: 'coming-soon/inheritance' },
            { label: 'Interfaces', slug: 'coming-soon/interfaces' },
            { label: 'Closures', slug: 'coming-soon/closures' },
            { label: 'Exception Handling', slug: 'coming-soon/exception-handling' },
          ],
        },
      ],
    }),
  ],
});

