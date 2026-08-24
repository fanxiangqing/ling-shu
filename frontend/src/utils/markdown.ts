import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const markdown = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true
})

const defaultLinkOpen = markdown.renderer.rules.link_open || ((tokens, index, options, env, self) => self.renderToken(tokens, index, options))

markdown.renderer.rules.link_open = (tokens, index, options, env, self) => {
  const token = tokens[index]
  const targetIndex = token.attrIndex('target')
  const relIndex = token.attrIndex('rel')
  if (targetIndex < 0) token.attrPush(['target', '_blank'])
  if (relIndex < 0) token.attrPush(['rel', 'noreferrer noopener'])
  return defaultLinkOpen(tokens, index, options, env, self)
}

export function renderMarkdown(value: string) {
  const source = value.trim()
  if (!source) return ''
  return DOMPurify.sanitize(markdown.render(source), {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['target', 'rel']
  })
}
