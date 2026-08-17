import React, { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Empty } from '@douyinfe/semi-ui';
import {
  IconAlertCircle,
  IconChevronLeft,
  IconChevronRight,
} from '@douyinfe/semi-icons';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import { docsByPath, docsNavigation, flatDocs, DOCS_URL } from './docs-data';
import './tutorial-docs.css';

const DEFAULT_DOC_PATH = 'intro';

function normalizeDocPath(value) {
  if (Array.isArray(value)) {
    return value
      .filter(Boolean)
      .join('/')
      .replace(/^\/+|\/+$/g, '');
  }
  if (typeof value === 'string') {
    return value.replace(/^\/+|\/+$/g, '');
  }
  return '';
}

function getDocUrl(path) {
  return path === DEFAULT_DOC_PATH
    ? '/tutorial-docs'
    : `/tutorial-docs/${path}`;
}

function DocNav({ activePath, onNavigate }) {
  return (
    <nav className='tutorial-docs-nav' aria-label='教程目录'>
      {docsNavigation.map((group) => (
        <section className='tutorial-docs-nav-group' key={group.title}>
          <h2>{group.title}</h2>
          <div className='tutorial-docs-nav-items'>
            {group.items.map((item) => (
              <Link
                className={
                  item.path === activePath
                    ? 'tutorial-docs-nav-link is-active'
                    : 'tutorial-docs-nav-link'
                }
                key={item.path}
                onClick={onNavigate}
                to={getDocUrl(item.path)}
              >
                {item.title}
              </Link>
            ))}
          </div>
        </section>
      ))}
    </nav>
  );
}

function PageToc({ sections, activeHeading, onSelect }) {
  if (!sections?.length) return null;

  return (
    <aside className='tutorial-docs-toc' aria-label='本页目录'>
      <h2>本页目录</h2>
      <div className='tutorial-docs-toc-list'>
        {sections.map((section) => (
          <button
            className={
              section.id === activeHeading
                ? 'tutorial-docs-toc-link is-active'
                : 'tutorial-docs-toc-link'
            }
            key={section.id}
            onClick={() => onSelect(section.id)}
            type='button'
          >
            {section.title}
          </button>
        ))}
      </div>
    </aside>
  );
}

function PrevNext({ currentIndex }) {
  const previousDoc = currentIndex > 0 ? flatDocs[currentIndex - 1] : null;
  const nextDoc =
    currentIndex >= 0 && currentIndex < flatDocs.length - 1
      ? flatDocs[currentIndex + 1]
      : null;

  if (!previousDoc && !nextDoc) return null;

  return (
    <nav className='tutorial-docs-pager' aria-label='上一篇和下一篇'>
      {previousDoc ? (
        <Link
          className='tutorial-docs-pager-card'
          to={getDocUrl(previousDoc.path)}
        >
          <span>
            <IconChevronLeft />
            上一篇
          </span>
          <strong>{previousDoc.title}</strong>
        </Link>
      ) : (
        <span className='tutorial-docs-pager-card is-placeholder' />
      )}
      {nextDoc ? (
        <Link
          className='tutorial-docs-pager-card is-next'
          to={getDocUrl(nextDoc.path)}
        >
          <span>
            下一篇
            <IconChevronRight />
          </span>
          <strong>{nextDoc.title}</strong>
        </Link>
      ) : (
        <span className='tutorial-docs-pager-card is-placeholder' />
      )}
    </nav>
  );
}

function MissingDoc({ requestedPath }) {
  return (
    <div className='tutorial-docs-missing'>
      <Empty
        image={<IconAlertCircle size='extra-large' />}
        description='文档不存在'
      >
        <p>
          没有找到
          <code>{requestedPath || '/tutorial-docs'}</code>
          对应的教程，请从左侧目录重新选择。
        </p>
        <Link className='tutorial-docs-primary-link' to='/tutorial-docs'>
          返回可乐AI 接入总览
        </Link>
      </Empty>
    </div>
  );
}

export default function TutorialDocs() {
  const params = useParams();
  const requestedPath = normalizeDocPath(params['*']);
  const activePath = requestedPath || DEFAULT_DOC_PATH;
  const doc = docsByPath[activePath];
  const currentIndex = flatDocs.findIndex((item) => item.path === activePath);
  const currentTitle =
    flatDocs.find((item) => item.path === activePath)?.title ||
    '可乐AI 教程文档';
  const [activeHeading, setActiveHeading] = useState(doc?.sections?.[0]?.id);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  const headings = useMemo(() => doc?.sections || [], [doc]);

  useEffect(() => {
    document.title = doc
      ? `${currentTitle} - 可乐AI 教程文档`
      : '文档不存在 - 可乐AI 教程文档';
  }, [currentTitle, doc]);

  useEffect(() => {
    setActiveHeading(headings[0]?.id);
    window.scrollTo({ top: 0, behavior: 'auto' });
  }, [activePath, headings]);

  useEffect(() => {
    if (!headings.length) return undefined;

    const updateActiveHeading = () => {
      let current = headings[0]?.id;

      for (const section of headings) {
        const element = document.getElementById(section.id);
        if (!element) continue;
        if (element.getBoundingClientRect().top <= 130) {
          current = section.id;
        } else {
          break;
        }
      }

      setActiveHeading(current);
    };

    updateActiveHeading();
    window.addEventListener('scroll', updateActiveHeading, { passive: true });

    return () => window.removeEventListener('scroll', updateActiveHeading);
  }, [headings]);

  const handleHeadingSelect = (id) => {
    const element = document.getElementById(id);
    if (!element) return;

    element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    window.history.replaceState(null, '', `#${id}`);
    setActiveHeading(id);
  };

  return (
    <div className='tutorial-docs-page'>
      <main className='tutorial-docs-container'>
        <header className='tutorial-docs-hero'>
          <p>教程文档</p>
          <h1>{doc ? currentTitle : '文档不存在'}</h1>
          <span>
            将可乐AI 接入说明整理为站内教程页，围绕 New API、客户端配置和 OpenAI
            Compatible 工作流提供快速指引。
          </span>
        </header>

        <details
          className='tutorial-docs-mobile-nav'
          onToggle={(event) => setMobileNavOpen(event.currentTarget.open)}
          open={mobileNavOpen}
        >
          <summary>教程目录</summary>
          <DocNav
            activePath={activePath}
            onNavigate={() => setMobileNavOpen(false)}
          />
        </details>

        <div className='tutorial-docs-layout'>
          <aside className='tutorial-docs-sidebar'>
            <DocNav activePath={activePath} />
          </aside>

          <article className='tutorial-docs-article'>
            {doc ? (
              <>
                <header className='tutorial-docs-article-header'>
                  <p>可乐AI 教程文档</p>
                  <h1>{currentTitle}</h1>
                  <span>{doc.description}</span>
                </header>

                {doc.sections.map((section) => (
                  <section className='tutorial-docs-section' key={section.id}>
                    <h2 id={section.id}>{section.title}</h2>
                    <MarkdownRenderer
                      className='tutorial-docs-markdown'
                      content={section.content}
                      fontSize={15}
                    />
                  </section>
                ))}

                <PrevNext currentIndex={currentIndex} />
              </>
            ) : (
              <MissingDoc requestedPath={activePath} />
            )}
          </article>

          <div className='tutorial-docs-toc-wrap'>
            {doc && (
              <PageToc
                activeHeading={activeHeading}
                onSelect={handleHeadingSelect}
                sections={doc.sections}
              />
            )}
          </div>
        </div>
      </main>
      <div className='tutorial-docs-footer-note'>
        后台“文档地址”填写：
        <code>{DOCS_URL}</code>
      </div>
    </div>
  );
}
