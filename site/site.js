(() => {
  const languageKey = 'n0-site-language';
  const themeKey = 'n0-site-theme';
  const page = document.body.dataset.page;

  const text = (selector, value, html = false) => {
    const element = document.querySelector(selector);
    if (element) html ? (element.innerHTML = value) : (element.textContent = value);
  };

  const pageCopy = {
    home: {
      nav: ['about profile', 'projects', 'writing', 'GitHub ↗'],
      eyebrow: 'sickagent · developer tools',
      title: 'Projects,<br /><em>writing, tools.</em>',
      hero: 'sickagent is a personal space for open source projects, engineering notes and tools at the intersection of AI, development and data.',
      actions: ['view projects ↓', 'open GitHub ↗'],
      labels: ['profile', 'projects', 'writing'],
      aboutTitle: 'Code, context<br />and results.',
      about: 'n0 is an execution and metadata layer between AI agents and data sources. It handles connections, tenant isolation, query validation, execution jobs and structured results.',
      aboutMuted: 'Stack: Go, REST API, PostgreSQL, NATS JetStream, Redis, S3-compatible storage and database adapters.',
      projectsTitle: 'Projects',
      projectsNote: 'Tools and systems<br />for real-world problems.',
      projectType: 'open source · active',
      project: 'AI-BI platform for safely connecting agents to PostgreSQL, MySQL, ClickHouse, SQLite, MSSQL and BigQuery through one execution layer.',
      projectLink: 'read about the project →',
      repoLink: 'open repository ↗',
      writingTitle: 'Writing<br />and notes',
      writingNote: 'Engineering notes<br />about how things work.',
      articleMeta: '01 / architecture · n0',
      articleTitle: 'How to give an AI agent data access without a direct database connection',
      footer: ['© sickagent', 'built with attention to detail', 'GitHub ↗']
    },
    project: {
      eyebrow: 'n0 · AI-BI platform · active',
      back: '← back to projects',
      lede: 'A secure execution layer between AI agents and corporate data sources.',
      label: 'problem',
      introTitle: 'Give agents<br />data without direct access.',
      intro: 'An AI agent can write SQL, but it should not receive permanent production credentials or connect directly to a database. n0 accepts an analytical task through the Agent Gateway, checks it against policies and runs it inside the Query Engine.',
      introMuted: 'The project is under active development. The core end-to-end flow, policy-enforced query execution, durable result storage, plugin health routing and audit persistence are already working.',
      featuresLabel: 'capabilities',
      featuresTitle: 'What n0 does',
      features: [['validates queries', 'Allows single SELECT statements, limits rows and blocks dangerous operations.'], ['runs async jobs', 'Sends analytical queries through NATS JetStream and exposes status polling.'], ['isolates tenants', 'Checks connection and job ownership on every public operation.']],
      ctaEyebrow: 'source code',
      ctaTitle: 'Architecture and<br />setup are in the docs.',
      docsLink: 'read the documentation →',
      githubLink: 'open the n0 repository ↗',
      footer: ['© sickagent', 'sickagent.']
    },
    docs: {
      back: '← n0 project page',
      version: 'AI-BI platform<br />Go 1.26 · active development',
      nav: ['Overview', 'Request flow', 'Use cases', 'Security'],
      eyebrow: 'project / documentation',
      lede: 'AI-BI execution layer for safely connecting AI agents to corporate data.',
      labels: ['overview', 'workflow', 'use cases', 'principles'],
      overviewTitle: 'A controlled layer<br />between agents and data.',
      overview: ['An AI agent can generate SQL, but it should not receive a permanent connection string, connect directly to production or handle access, retries, timeouts and audit on its own.', 'n0 accepts a request through the Agent Gateway, adds tenant context, validates it against policies, puts it into an asynchronous queue and returns structured data for analysis, charts or reports.'],
      note: ['In short', 'The agent decides what to ask. n0 controls who can access data, which query is allowed and how it runs.'],
      workflowTitle: 'The analytical request path',
      workflow: [['Gateway', 'An agent or Web Admin sends a REST API request. The Gateway validates JWT, workspace and tenant context.'], ['Query guardrails', 'n0 accepts a single SELECT, checks table access, adds limits and blocks DDL/DML.'], ['JetStream job', 'The request becomes an asynchronous job. Query Engine workers run it with bounded timeouts and controlled retries.'], ['Result and audit', 'The client receives status and paginated results. Metadata and large payloads are stored, while actions enter the audit pipeline.']],
      casesTitle: 'Real integration<br />scenarios',
      cases: [['AI data analyst', 'A corporate agent explores available schemas and answers data questions without a direct connection string.', 'Result →', 'SQL and data pass through one access and validation boundary.'], ['Embedded analytics', 'A product submits analytical jobs through a stable API and receives structured rows for its own UI.', 'Result →', 'Charts and dashboards remain on the product side.'], ['Self-service data access', 'Teams register approved sources and run restricted read-only queries through one gateway.', 'Result →', 'Connections and jobs are isolated by tenant_id.'], ['Adapter ecosystem', 'A team adds a specific DWH or corporate source through an adapter or plugin without changing API consumers.', 'Result →', 'New sources join the same execution-layer model.']],
      securityTitle: 'Security<br />and boundaries',
      security: [['Zero trust for agents', 'Agents do not receive direct database access and are not treated as trusted parties.'], ['Tenant isolation', 'Connection and job ownership are checked on every public operation.'], ['Query sandbox', 'Single SELECT, table allowlist, tenant predicate injection and LIMIT control.'], ['Credential protection', 'Connection parameters are encrypted with AES-256-GCM and never returned to clients.']],
      footerCta: 'Source code and development status',
      githubLink: 'open the n0 repository ↗',
      footer: ['© sickagent', 'sickagent.']
    }
  };

  function applyEnglish() {
    const copy = pageCopy[page];
    if (!copy) return;
    document.documentElement.lang = 'en';
    document.title = page === 'home' ? 'sickagent — projects, writing, tools' : page === 'project' ? 'n0 — sickagent project' : 'n0 — documentation';
    const description = document.querySelector('meta[name="description"]');
    if (description) description.content = page === 'home' ? 'sickagent — projects, writing and tools for AI, development and data.' : page === 'project' ? 'n0 — a Go-based AI-BI platform for safely connecting AI agents to corporate data.' : 'n0 documentation: architecture, request flow, security and integration scenarios.';
    if (page === 'home') {
      text('nav a[href="#about"]', copy.nav[0]); text('nav a[href="#projects"]', copy.nav[1]); text('nav a[href="#writing"]', copy.nav[2]); text('.nav-github', copy.nav[3]);
      text('.hero .eyebrow', copy.eyebrow); text('.hero h1', copy.title, true); text('.hero-copy', copy.hero); text('.hero-actions .button', copy.actions[0]); text('.hero-actions .text-link', copy.actions[1]);
      document.querySelectorAll('.about .section-label, .projects .section-label, .writing .section-label')[0].lastChild.textContent = ` ${copy.labels[0]}`;
      document.querySelectorAll('.about .section-label, .projects .section-label, .writing .section-label')[1].lastChild.textContent = ` ${copy.labels[1]}`;
      document.querySelectorAll('.about .section-label, .projects .section-label, .writing .section-label')[2].lastChild.textContent = ` ${copy.labels[2]}`;
      text('#about-title', copy.aboutTitle, true); text('.about-copy p:first-child', copy.about); text('.about-copy .muted', copy.aboutMuted); text('#projects-title', copy.projectsTitle); text('.projects-heading p', copy.projectsNote, true); text('.project-kind', copy.projectType); text('.project-main p', copy.project); text('.project-link', copy.projectLink); text('.repo-link', copy.repoLink); text('#writing-title', copy.writingTitle, true); text('.writing-heading p', copy.writingNote, true); text('.article-meta', copy.articleMeta); text('.article-title', copy.articleTitle); document.querySelectorAll('.site-footer span, .site-footer a').forEach((el, i) => { if (copy.footer[i]) el.textContent = copy.footer[i]; });
    }
    if (page === 'project') {
      text('.project-page > .eyebrow', copy.eyebrow); text('.back-link', copy.back); text('.project-lede', copy.lede); text('.project-intro-grid .section-label', `01 ${copy.label}`); text('.project-intro-grid h2', copy.introTitle, true); text('.long-copy p:first-child', copy.intro); text('.long-copy p:last-child', copy.introMuted); text('.principles .section-label', `02 ${copy.featuresLabel}`); text('#principles-title', copy.featuresTitle); document.querySelectorAll('.principle-grid article').forEach((el, i) => { text(`.principle-grid article:nth-child(${i + 1}) h3`, copy.features[i][0]); text(`.principle-grid article:nth-child(${i + 1}) p`, copy.features[i][1]); }); text('.project-cta .eyebrow', copy.ctaEyebrow); text('.project-cta h2', copy.ctaTitle, true); text('.cta-actions .button', copy.docsLink); text('.cta-actions .text-link', copy.githubLink); document.querySelectorAll('.site-footer span, .site-footer a').forEach((el, i) => { if (copy.footer[i]) el.textContent = copy.footer[i]; });
    }
    if (page === 'docs') {
      text('.back-link', copy.back); text('.docs-version', copy.version, true); document.querySelectorAll('.docs-nav a').forEach((el, i) => { el.textContent = copy.nav[i]; }); text('.docs-content > .eyebrow', copy.eyebrow); text('.docs-lede', copy.lede); document.querySelectorAll('.docs-section .section-label').forEach((el, i) => { el.lastChild.textContent = ` ${copy.labels[i]}`; }); text('#overview h2', copy.overviewTitle, true); text('#overview > p:nth-of-type(1)', copy.overview[0]); text('#overview > p:nth-of-type(2)', copy.overview[1]); text('.docs-note strong', copy.note[0]); text('.docs-note span', copy.note[1]); text('#workflow h2', copy.workflowTitle); document.querySelectorAll('.workflow-list li').forEach((el, i) => { text(`#workflow li:nth-child(${i + 1}) h3`, copy.workflow[i][0]); text(`#workflow li:nth-child(${i + 1}) p`, copy.workflow[i][1]); }); text('#cases h2', copy.casesTitle, true); document.querySelectorAll('.case-card').forEach((el, i) => { text(`#cases .case-card:nth-child(${i + 1}) h3`, copy.cases[i][0]); text(`#cases .case-card:nth-child(${i + 1}) p`, copy.cases[i][1]); text(`#cases .case-card:nth-child(${i + 1}) strong`, copy.cases[i][2]); text(`#cases .case-card:nth-child(${i + 1}) em`, copy.cases[i][3]); }); text('#principles h2', copy.securityTitle, true); document.querySelectorAll('.principle-rows div').forEach((el, i) => { text(`#principles .principle-rows div:nth-child(${i + 1}) strong`, copy.security[i][0]); text(`#principles .principle-rows div:nth-child(${i + 1}) span`, copy.security[i][1]); }); text('.docs-footer-cta p', copy.footerCta); text('.docs-footer-cta a', copy.githubLink); document.querySelectorAll('.site-footer span, .site-footer a').forEach((el, i) => { if (copy.footer[i]) el.textContent = copy.footer[i]; });
    }
  }

  function setLanguage(language) {
    localStorage.setItem(languageKey, language);
    if (language === 'en') applyEnglish();
    else if (document.documentElement.lang === 'en') { window.location.reload(); return; }
    document.querySelectorAll('[data-language-toggle]').forEach((button) => { button.textContent = language === 'en' ? 'RU' : 'EN'; button.setAttribute('aria-label', language === 'en' ? 'Switch to Russian' : 'Switch to English'); });
  }

  function setTheme(theme) {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem(themeKey, theme);
    document.querySelectorAll('[data-theme-toggle]').forEach((button) => { button.textContent = theme === 'dark' ? '☼' : '◐'; button.setAttribute('aria-label', theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'); });
  }

  const storedLanguage = localStorage.getItem(languageKey);
  const language = storedLanguage || ((navigator.language || '').toLowerCase().startsWith('en') ? 'en' : 'ru');
  const storedTheme = localStorage.getItem(themeKey);
  const theme = storedTheme || (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  setLanguage(language); setTheme(theme);
  document.querySelectorAll('[data-language-toggle]').forEach((button) => button.addEventListener('click', () => setLanguage(localStorage.getItem(languageKey) === 'en' ? 'ru' : 'en')));
  document.querySelectorAll('[data-theme-toggle]').forEach((button) => button.addEventListener('click', () => setTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark')));
})();
