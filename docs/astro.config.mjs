import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Zeus TextMate grammar for syntax highlighting
const zeusGrammar = {
  name: 'zeus',
  scopeName: 'source.zeus',
  patterns: [
    {
      name: 'keyword.control.zeus',
      match: '\\b(if|else|for|while|break|continue|return|new|class|extends|super|private|protected|public|static|get|set|this|try|throw|catch|finally|constructor)\\b'
    },
    {
      name: 'storage.type.zeus',
      match: '\\b(type|function)\\b'
    },
    {
      name: 'storage.modifier.zeus',
      match: '\\b(let|const)\\b'
    },
    {
      name: 'entity.name.tag.zeus',
      match: '@[a-zA-Z_][a-zA-Z0-9_]*'
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
      match: '\\b(i8|i16|i32|i64|i128|u8|u16|u32|u64|f16|f32|f64|f128|boolean|void|string|null|cint|clong|csize|cptr|cstr|cdouble)\\b'
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
          label: 'Getting Started',
          items: [
            { label: 'Why Zeus?', slug: 'why-zeus' },
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Hello World', slug: 'getting-started/hello-world' },
            { label: 'Editor Setup', slug: 'getting-started/ide-setup' },
          ],
        },
        {
          label: 'Language Guide',
          items: [
            { label: 'Variables & Constants', slug: 'language/variables' },
            { label: 'Types', slug: 'language/types' },
            { label: 'Operators', slug: 'language/operators' },
            { label: 'Control Flow', slug: 'language/control-flow' },
            { label: 'Functions', slug: 'language/functions' },
            { label: 'Closures', slug: 'language/closures' },
            { label: 'Arrays', slug: 'language/arrays' },
            { label: 'Strings', slug: 'language/strings' },
            { label: 'Modules', slug: 'language/modules' },
            { label: 'Exception Handling', slug: 'language/exception-handling' },
          ],
        },
        {
          label: 'Classes & OOP',
          items: [
            { label: 'Classes', slug: 'language/classes' },
            { label: 'Encapsulation & Accessors', slug: 'language/encapsulation' },
            { label: 'Static Members', slug: 'language/static-members' },
            { label: 'Inheritance', slug: 'language/inheritance' },
            { label: 'Interfaces', slug: 'language/interfaces' },
          ],
        },
        {
          label: 'C Interop',
          items: [
            { label: 'Foreign Function Interface', slug: 'c-interop/ffi' },
            { label: 'Linking Libraries', slug: 'c-interop/linking' },
          ],
        },
        {
          label: 'Standard Library',
          items: [
            { label: 'Console', slug: 'language/console' },
            { label: 'Math', slug: 'language/math' },
            { label: 'Colors', slug: 'language/colors' },
            { label: 'Timers', slug: 'language/timers' },
            { label: 'Number Parsing', slug: 'stdlib/numbers' },
            { label: 'JSON', slug: 'stdlib/json' },
            { label: 'process', slug: 'stdlib/process' },
            { label: 'File System (fs)', slug: 'stdlib/fs' },
            { label: 'Operating System (os)', slug: 'stdlib/os' },
            { label: 'Path (path)', slug: 'stdlib/path' },
            { label: 'Date & Time', slug: 'stdlib/datetime' },
            { label: 'Buffer', slug: 'stdlib/buffer' },
            { label: 'Encoding', slug: 'stdlib/encoding' },
            { label: 'Crypto', slug: 'stdlib/crypto' },
          ],
        },
        {
          label: 'Tooling',
          items: [
            { label: 'Compiling Programs', slug: 'tooling/release-builds' },
            { label: 'Formatter', slug: 'tooling/formatter' },
            { label: 'Testing', slug: 'tooling/testing' },
            { label: 'Language Server', slug: 'tooling/lsp' },
          ],
        },
        {
          label: 'Performance',
          items: [
            { label: 'Benchmarks', slug: 'performance/benchmarks' },
          ],
        },
        {
          label: 'Roadmap',
          badge: { text: 'Planned', variant: 'note' },
          items: [
            { label: 'What\'s next', slug: 'roadmap' },
          ],
        },
        {
          label: 'Contributing',
          items: [
            { label: 'Building from Source', slug: 'developer/building-from-source' },
          ],
        },
      ],
    }),
  ],
});

