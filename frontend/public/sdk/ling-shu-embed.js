(function () {
  var activeInstance = null

  function scriptOrigin() {
    var script = document.currentScript
    if (script && script.src) return new URL(script.src).origin
    return window.location.origin
  }

  function normalizeToken(value) {
    if (!value) return ''
    if (typeof value === 'string') return value
    if (value.access_token) return value.access_token
    if (value.data && value.data.access_token) return value.data.access_token
    return ''
  }

  function createElement(tag, className) {
    var element = document.createElement(tag)
    if (className) element.className = className
    return element
  }

  function launcherIconSVG() {
    return [
      '<svg viewBox="0 0 40 40" role="img" aria-label="Ling-Shu">',
      '<path class="ls-icon-spark" d="M29.5 7.5l1.6 3.2 3.4 1.2-3.4 1.2-1.6 3.2-1.6-3.2-3.4-1.2 3.4-1.2 1.6-3.2z"/>',
      '<path class="ls-icon-stem" d="M20 9v5"/>',
      '<rect class="ls-icon-face" x="9" y="14" width="22" height="20" rx="7"/>',
      '<path class="ls-icon-ear" d="M9 23H6.5M33.5 23H31"/>',
      '<circle class="ls-icon-eye" cx="16" cy="23" r="1.7"/>',
      '<circle class="ls-icon-eye" cx="24" cy="23" r="1.7"/>',
      '<path class="ls-icon-chart" d="M15 29h10M16.5 29v-3M20 29v-5M23.5 29v-2"/>',
      '</svg>'
    ].join('')
  }

  function mountStyles(root) {
    var style = document.createElement('style')
    style.textContent = [
      ':host{all:initial}',
      '.ls-root{font-family:"Aptos","PingFang SC","Microsoft YaHei",sans-serif;color:#17211c}',
      '.ls-launcher{position:fixed;z-index:2147483000;display:flex;align-items:center;gap:11px;border:1px solid rgba(255,255,255,.14);border-radius:999px;padding:8px 16px 8px 8px;background:linear-gradient(135deg,#0d251a,#102019 58%,#0a1712);color:#fff;box-shadow:0 18px 42px rgba(16,32,25,.3),inset 0 1px 0 rgba(255,255,255,.08);cursor:pointer;font:800 14px/1.1 inherit;letter-spacing:0;transition:transform .18s ease,box-shadow .18s ease,border-color .18s ease}',
      '.ls-launcher:hover{transform:translateY(-2px);border-color:rgba(49,207,154,.38);box-shadow:0 22px 52px rgba(16,32,25,.36),0 0 0 6px rgba(38,179,134,.1)}',
      '.ls-launcher.bottom-right{right:24px;bottom:24px}',
      '.ls-launcher.bottom-left{left:24px;bottom:24px}',
      '.ls-launcher.top-right{right:24px;top:24px}',
      '.ls-launcher.top-left{left:24px;top:24px}',
      '.ls-orb{width:42px;height:42px;border-radius:999px;display:grid;place-items:center;background:radial-gradient(circle at 32% 24%,#65e8bd,#24ba8c 58%,#0d7d60);color:#f7fffb;box-shadow:inset 0 0 0 1px rgba(255,255,255,.36),0 8px 18px rgba(30,180,134,.28)}',
      '.ls-orb svg{display:block;width:28px;height:28px;overflow:visible}',
      '.ls-icon-face,.ls-icon-stem,.ls-icon-ear,.ls-icon-chart{fill:none;stroke:currentColor;stroke-width:2.2;stroke-linecap:round;stroke-linejoin:round}',
      '.ls-icon-eye,.ls-icon-spark{fill:currentColor}',
      '.ls-label{white-space:nowrap}',
      '.ls-panel{position:fixed;z-index:2147483001;width:min(520px,calc(100vw - 32px));height:min(760px,calc(100vh - 112px));border-radius:22px;overflow:hidden;background:#fbfaf6;box-shadow:0 30px 80px rgba(18,28,23,.3);border:1px solid rgba(12,45,31,.16);display:none}',
      '.ls-panel.open{display:block}',
      '.ls-panel.bottom-right{right:24px;bottom:94px}',
      '.ls-panel.bottom-left{left:24px;bottom:94px}',
      '.ls-panel.top-right{right:24px;top:94px}',
      '.ls-panel.top-left{left:24px;top:94px}',
      '.ls-frame{width:100%;height:100%;border:0;background:#fbfaf6}',
      '.ls-close{position:absolute;right:10px;top:10px;z-index:2;width:32px;height:32px;border:0;border-radius:999px;background:rgba(15,31,23,.08);color:#11231a;cursor:pointer;font:700 18px/1 inherit}',
      '.ls-loading{position:absolute;inset:0;display:grid;place-items:center;background:#fbfaf6;color:#44524a;font:700 14px/1.4 inherit}',
      '@media (max-width:520px){.ls-launcher{right:16px!important;left:auto!important;bottom:16px!important;top:auto!important}.ls-panel{inset:10px!important;width:auto;height:auto;border-radius:14px}.ls-label{display:none}}'
    ].join('')
    root.appendChild(style)
  }

  function createInstance(options) {
    if (!options || !options.appId) throw new Error('LingShuEmbed: appId is required')
    if (typeof options.tokenProvider !== 'function') throw new Error('LingShuEmbed: tokenProvider is required')

    var baseUrl = (options.baseUrl || scriptOrigin()).replace(/\/$/, '')
    var position = options.position || 'bottom-right'
    var key = options.key || 'default'
    var sessionMode = options.sessionMode || ''
    var parentOrigin = options.parentOrigin || window.location.origin
    var launcherTitle = (options.launcher && options.launcher.title) || options.title || '智能问数'
    var host = createElement('div')
    var shadow = host.attachShadow({ mode: 'open' })
    var root = createElement('div', 'ls-root')
    var launcher = createElement('button', 'ls-launcher ' + position)
    var orb = createElement('span', 'ls-orb')
    var label = createElement('span', 'ls-label')
    var panel = createElement('section', 'ls-panel ' + position)
    var close = createElement('button', 'ls-close')
    var loading = createElement('div', 'ls-loading')
    var iframe = null
    var opened = false

    orb.innerHTML = launcherIconSVG()
    label.textContent = launcherTitle
    close.type = 'button'
    close.textContent = 'x'
    loading.textContent = '正在打开助手'
    launcher.type = 'button'
    launcher.setAttribute('aria-label', launcherTitle)
    launcher.appendChild(orb)
    launcher.appendChild(label)
    panel.appendChild(close)
    panel.appendChild(loading)
    root.appendChild(launcher)
    root.appendChild(panel)
    mountStyles(shadow)
    shadow.appendChild(root)
    document.body.appendChild(host)

    async function ensureFrame() {
      if (iframe) return iframe
      loading.style.display = 'grid'
      var token = normalizeToken(await options.tokenProvider())
      if (!token) throw new Error('LingShuEmbed: tokenProvider did not return an access token')
      var params = new URLSearchParams()
      params.set('key', key)
      params.set('parent_origin', parentOrigin)
      if (sessionMode) params.set('session_mode', sessionMode)
      iframe = createElement('iframe', 'ls-frame')
      iframe.allow = 'microphone; autoplay'
      iframe.referrerPolicy = 'origin'
      iframe.src = baseUrl + '/embed/' + encodeURIComponent(options.appId) + '?' + params.toString() + '#token=' + encodeURIComponent(token)
      iframe.onload = function () {
        loading.style.display = 'none'
      }
      panel.appendChild(iframe)
      return iframe
    }

    async function open() {
      panel.classList.add('open')
      opened = true
      try {
        await ensureFrame()
      } catch (error) {
        loading.textContent = error && error.message ? error.message : '助手打开失败'
      }
    }

    function closePanel() {
      panel.classList.remove('open')
      opened = false
    }

    function toggle() {
      if (opened) {
        closePanel()
        return
      }
      void open()
    }

    launcher.addEventListener('click', toggle)
    close.addEventListener('click', closePanel)
    if (options.autoOpen) {
      setTimeout(function () {
        void open()
      }, 0)
    }

    return {
      open: open,
      close: closePanel,
      destroy: function () {
        launcher.removeEventListener('click', toggle)
        close.removeEventListener('click', closePanel)
        if (host.parentNode) host.parentNode.removeChild(host)
      }
    }
  }

  window.LingShuEmbed = {
    init: function (options) {
      if (activeInstance) activeInstance.destroy()
      activeInstance = createInstance(options)
      return activeInstance
    }
  }
})()
