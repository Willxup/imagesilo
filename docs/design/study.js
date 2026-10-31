const icon = (name, cls = '') => `<svg${cls ? ` class="${cls}"` : ''} aria-hidden="true"><use href="#${name}"/></svg>`
const images = [
  { id: 12, name: '海岸的午后.jpg', dimensions: '2500 × 1667', size: '1.8 MB', date: '今天 14:32', private: false },
  { id: 14, name: '沿着海岸线.jpg', dimensions: '2500 × 1667', size: '2.1 MB', date: '今天 14:28', private: false },
  { id: 15, name: '山谷里的水声.jpg', dimensions: '2500 × 1667', size: '1.4 MB', date: '今天 11:05', private: true },
  { id: 17, name: '北方的风景.jpg', dimensions: '2500 × 1667', size: '2.6 MB', date: '昨天 18:42', private: false },
  { id: 18, name: '日光经过的地方.jpg', dimensions: '2500 × 1667', size: '1.9 MB', date: '昨天 16:20', private: false },
  { id: 20, name: '工作台的一角.jpg', dimensions: '3670 × 2462', size: '980 KB', date: '昨天 10:16', private: true },
]
const names = { library: '图片库', upload: '上传图片', migration: '迁移文件', aliases: '路径映射', tokens: 'API 令牌', settings: '设置', system: '系统状态', detail: '图片详情' }
const main = document.querySelector('#main')
const selected = new Set()
let page = 'library', direction = 'studio', visibility = 'all', view = 'grid', filtersOpen = false, activeImage = null, queue = [], uploadTimer, toastTimer, openMenuElement, menuAnchor, linkFormat = '直链'
const reducedMotion = () => matchMedia('(prefers-reduced-motion: reduce)').matches

function transition(update) {
  if (document.startViewTransition && !reducedMotion()) return document.startViewTransition(update)
  update()
}

function heading(title, description, action = '', eyebrow = '我的图片空间') {
  return `<div class="page-heading"><div><span class="eyebrow">${eyebrow}</span><h1>${title}</h1><p>${description}</p></div>${action}</div>`
}

function renderLibrary() {
  const current = images.filter(image => visibility === 'all' || image.private === (visibility === 'private'))
  main.innerHTML = `${heading('图片库 <span>6</span>', '收好每一张图片，随时找到，轻松分享。', `<button class="button primary" data-go="upload">${icon('plus')}上传图片</button>`)}
    <div class="library-toolbar"><div class="library-tabs" role="tablist" aria-label="图片可见性"><button role="tab" aria-selected="${visibility === 'all'}" data-visibility="all" class="${visibility === 'all' ? 'active' : ''}">全部图片</button><button role="tab" aria-selected="${visibility === 'public'}" data-visibility="public" class="${visibility === 'public' ? 'active' : ''}">公开</button><button role="tab" aria-selected="${visibility === 'private'}" data-visibility="private" class="${visibility === 'private' ? 'active' : ''}">私密</button></div><div class="library-tools"><button class="button filter-button ${filtersOpen ? 'active' : ''}" id="filter-toggle" aria-label="筛选图片" aria-expanded="${filtersOpen}">${icon('filter')}<span>筛选</span></button><div class="view-toggle" role="group" aria-label="视图"><button data-view="grid" aria-label="网格视图" class="${view === 'grid' ? 'active' : ''}">${icon('grid')}</button><button data-view="list" aria-label="列表视图" class="${view === 'list' ? 'active' : ''}">${icon('list')}</button></div></div></div>
    <div class="filters ${filtersOpen ? 'open' : ''}" ${filtersOpen ? '' : 'inert'}><div class="filters-inner"><form class="filter-fields" id="filter-form"><label>搜索图片<input class="field" placeholder="文件名、路径或 ID" name="search"></label><label>格式<select class="field"><option>所有格式</option><option>JPEG</option><option>PNG</option><option>WebP</option></select></label><label>上传来源<select class="field"><option>所有来源</option><option>管理后台</option><option>API 令牌</option></select></label><button class="button" type="submit">${icon('search')}搜索</button></form></div></div>
    <div class="grid ${view === 'list' ? 'list' : ''}">${current.map((image, i) => tile(image, i)).join('')}</div><div class="library-end">已显示全部 ${current.length} 张图片</div>`
  bindLibrary()
}

function tile(image, i) {
  return `<article class="image-tile ${selected.has(image.id) ? 'selected' : ''}" style="--i:${i}" data-id="${image.id}"><button class="photo-link" data-open="${image.id}" aria-label="查看 ${image.name}"><img src="assets/sample-${image.id}.jpg" alt="${image.name}" ${activeImage?.id === image.id ? 'style="view-transition-name:active-image"' : ''}></button><label class="image-select"><input type="checkbox" aria-label="选择 ${image.name}" value="${image.id}" ${selected.has(image.id) ? 'checked' : ''}>${icon('check')}</label>${image.private ? `<span class="private-mark">${icon('lock')}私密</span>` : ''}<div class="image-meta"><div><h2>${image.name}</h2><p>${image.dimensions} · ${image.size}</p></div><button class="tile-menu" data-copy="${image.id}" aria-label="复制 ${image.name} 链接">${icon('copy')}</button></div></article>`
}

function bindLibrary() {
  main.querySelectorAll('[data-open]').forEach(button => button.addEventListener('click', () => {
    activeImage = images.find(image => image.id === Number(button.dataset.open))
    button.querySelector('img').style.viewTransitionName = 'active-image'
    go('detail')
  }))
  main.querySelectorAll('.image-select input').forEach(input => input.addEventListener('change', () => {
    const id = Number(input.value)
    if (input.checked) selected.add(id)
    else selected.delete(id)
    input.closest('article').classList.toggle('selected', input.checked)
    updateBatch()
  }))
  main.querySelectorAll('[data-copy]').forEach(button => button.addEventListener('click', () => notify('示例：链接已复制')))
  main.querySelectorAll('[data-visibility]').forEach(button => button.addEventListener('click', () => { visibility = button.dataset.visibility; transition(renderLibrary) }))
  main.querySelectorAll('[data-view]').forEach(button => button.addEventListener('click', () => { view = button.dataset.view; transition(renderLibrary) }))
  main.querySelector('#filter-toggle').addEventListener('click', event => {
    filtersOpen = !filtersOpen
    const panel = main.querySelector('.filters')
    panel.classList.toggle('open', filtersOpen)
    panel.inert = !filtersOpen
    event.currentTarget.setAttribute('aria-expanded', String(filtersOpen))
    event.currentTarget.classList.toggle('active', filtersOpen)
  })
  main.querySelector('#filter-form').addEventListener('submit', event => {
    event.preventDefault()
    const query = new FormData(event.currentTarget).get('search').trim()
    const result = images.filter(image => image.name.includes(query))
    main.querySelector('.grid').innerHTML = result.map(tile).join('')
    main.querySelector('.library-end').textContent = `找到 ${result.length} 张图片 · 原型搜索示例`
    main.querySelectorAll('[data-open]').forEach(button => button.addEventListener('click', () => { activeImage = images.find(image => image.id === Number(button.dataset.open)); go('detail') }))
  })
  bindNavigation()
}

function renderUpload() {
  main.innerHTML = `${heading('上传图片', '留住灵感，从一张图片开始。', `<button class="button text" data-go="library">打开图片库 ${icon('arrow')}</button>`, '收集新的灵感')}
    <div class="upload-studio"><div class="drop-stage"><div class="fold-art" aria-hidden="true"><span></span><span></span><span></span></div><h2>把图片放到这里</h2><p>拖入文件，或直接粘贴剪贴板中的图片</p><button class="button" id="add-samples">${icon('plus')}选择图片</button><span class="drop-formats">JPG · PNG · WEBP · GIF</span></div><aside class="upload-options"><div><span class="section-label">上传选项</span><h3>按你的方式保存</h3></div><label>图片可见性<select class="field"><option>使用系统默认值 · 公开</option><option>公开</option><option>私密</option></select></label><p class="option-hint">公开图片可以通过链接直接访问。<br>私密图片仅供授权访问。</p><div class="option-rule"></div><dl><dt>单次最多</dt><dd>20 张</dd><dt>图片处理</dt><dd>保留原图</dd></dl><button class="button primary" id="start-upload" ${queue.length ? '' : 'disabled'}>${icon('upload')}上传 ${queue.length} 张图片</button></aside></div>
    <div class="queue-heading"><h2>上传队列 <small>${queue.length ? `· ${queue.length} 张图片` : ''}</small></h2><button class="button text" id="clear-queue" ${queue.length ? '' : 'hidden'}>清空队列</button></div><div id="queue">${queue.length ? queue.map(queueRow).join('') : `<div class="queue-empty">${icon('image')}选好图片后，它们会出现在这里</div>`}</div><div class="workflow-note"><span>${icon('lock')}文件保存在你自己的空间</span><span>${icon('link')}上传后即可复制链接</span></div>`
  bindNavigation()
  document.querySelector('#add-samples').addEventListener('click', () => { if (uploadTimer) return; queue = images.slice(0, 3).map(image => ({...image, progress: 0})); renderUpload(); notify('已加入 3 张示例图片') })
  document.querySelector('#clear-queue').addEventListener('click', () => { clearInterval(uploadTimer); uploadTimer = null; queue = []; renderUpload() })
  document.querySelector('#start-upload').addEventListener('click', startUpload)
  const stage = main.querySelector('.drop-stage')
  stage.addEventListener('dragover', event => { event.preventDefault(); stage.classList.add('dragging') })
  stage.addEventListener('dragleave', () => stage.classList.remove('dragging'))
  stage.addEventListener('drop', event => { event.preventDefault(); stage.classList.remove('dragging'); document.querySelector('#add-samples').click() })
  main.querySelectorAll('[data-result-copy]').forEach(button => button.addEventListener('click', () => notify('示例：链接已复制')))
}

function queueRow(image) {
  const complete = image.progress >= 100
  return `<article class="queue-row"><img class="queue-thumbnail" src="assets/sample-${image.id}.jpg" alt=""><div class="queue-body"><h3>${image.name}</h3><p>${image.size} · ${complete ? '已保存' : image.progress ? '正在上传' : '等待上传'}</p>${!complete ? `<div class="progress-track"><div class="progress-fill" style="width:${image.progress}%"></div></div>` : ''}</div><span class="queue-status ${complete ? 'complete' : ''}">${complete ? `${icon('check')}已完成` : `${image.progress}%`}</span>${complete ? `<button class="button" data-result-copy>${icon('copy')}复制链接</button>` : ''}</article>`
}

function startUpload() {
  if (uploadTimer || !queue.length) return
  const button = main.querySelector('#start-upload')
  button.disabled = true
  button.textContent = '正在上传…'
  main.querySelector('#clear-queue').disabled = true
  let tick = 0
  uploadTimer = setInterval(() => {
    tick++
    queue = queue.map((image, i) => ({...image, progress: Math.max(0, Math.min(100, (tick - i * 2) * 8))}))
    if (page === 'upload') main.querySelectorAll('.queue-row').forEach((row, i) => {
      const item = queue[i]
      const fill = row.querySelector('.progress-fill')
      if (fill) fill.style.width = `${item.progress}%`
      row.querySelector('.queue-status').textContent = `${item.progress}%`
      row.querySelector('.queue-body p').textContent = `${item.size} · ${item.progress ? '正在上传' : '等待上传'}`
    })
    if (queue.every(image => image.progress === 100)) {
      clearInterval(uploadTimer); uploadTimer = null
      if (page === 'upload') renderUpload()
      notify('3 张图片已上传 · 动效示例')
    }
  }, 180)
}

function renderDetail() {
  const image = activeImage || images[0]
  main.innerHTML = `<button class="detail-back" id="back-to-library">${icon('back')}返回图片库</button><div class="detail-layout"><div><div class="detail-stage"><img src="assets/sample-${image.id}.jpg" alt="${image.name}" style="view-transition-name:active-image"></div><div class="detail-under"><span>${image.dimensions} px</span><span>原图比例 · 适应窗口</span></div></div><aside class="detail-inspector"><span class="eyebrow">图片详情</span><h1>${image.name}</h1><p class="file-kind">JPEG 图片 · ${image.size}</p><div class="detail-visibility"><span>可见性</span><button class="status-pill" id="detail-privacy">${image.private ? '私密' : '公开'} ${icon('chevron')}</button></div><dl class="detail-info"><dt>上传时间</dt><dd>${image.date}</dd><dt>上传来源</dt><dd>管理后台</dd><dt>存储大小</dt><dd>${image.size}</dd><dt>尺寸</dt><dd>${image.dimensions}</dd></dl><div class="detail-actions"><button class="button primary" id="detail-copy">${icon('copy')}复制图片链接</button><button class="button" id="detail-format">直链 ${icon('chevron')}</button><button class="button" id="detail-open">${icon('image')}查看原图</button></div><section class="detail-section"><h2>历史路径</h2><p>为图片保留一个熟悉的地址。<br>原有链接也能继续使用。</p><label><span class="eyebrow" style="margin-top:16px;margin-bottom:0">添加路径</span><input class="field" placeholder="/images/my-photo.jpg"></label><button class="button" id="detail-alias">${icon('plus')}添加路径映射</button></section><button class="detail-danger" id="detail-delete">删除这张图片</button></aside></div>`
  main.querySelector('#back-to-library').addEventListener('click', () => go('library'))
  main.querySelector('#detail-copy').addEventListener('click', () => notify('示例：图片链接已复制'))
  main.querySelector('#detail-format').innerHTML = linkFormat + icon('chevron')
  main.querySelector('#detail-format').addEventListener('click', event => {
    const anchor = event.currentTarget
    openPrototypeMenu(anchor, ['直链', 'Markdown', 'BBCode', 'HTML'].map(format => ({label:format,selected:format===linkFormat,action:()=>{linkFormat=format;anchor.innerHTML=format+icon('chevron')}})))
  })
  main.querySelector('#detail-privacy').addEventListener('click', event => {
    const anchor = event.currentTarget
    openPrototypeMenu(anchor, [{label:'公开',selected:!image.private,action:()=>{image.private=false;anchor.innerHTML='公开 '+icon('chevron');notify('示例图片已改为公开')}},{label:'私密',selected:image.private,action:()=>{image.private=true;anchor.innerHTML='私密 '+icon('chevron');notify('示例图片已改为私密')}}])
  })
  main.querySelector('#detail-open').addEventListener('click', () => notify('示例：实际版本会打开原图'))
  main.querySelector('#detail-alias').addEventListener('click', () => notify('示例：保留原有路径映射功能'))
  main.querySelector('#detail-delete').addEventListener('click', () => notify('示例：实际删除前需要二次确认'))
}

function renderSettings() {
  main.innerHTML = `${heading('设置', '让图片空间更适合你的习惯。', '', '空间管理')}<div class="settings-layout"><nav class="settings-nav" aria-label="设置分区"><button class="active" data-section="profile">个人资料</button><button data-section="defaults">上传与处理</button><button data-section="security">账号安全</button></nav><div class="settings-content"><section class="setting-section" id="profile"><h2>个人资料</h2><p>你在这个空间中的名字，以及登录时使用的邮箱。</p><div class="profile-avatar"><span>W</span><div><p>Will</p><small>空间管理员</small></div></div><div class="settings-fields"><label>显示名称<input class="field" value="Will"></label><label>邮箱地址<input class="field" value="will@example.com" type="email"></label></div><button class="button primary" data-demo-save>保存资料</button></section><section class="setting-section" id="defaults"><h2>上传与处理</h2><p>选择默认行为，上传时也可以单独调整。</p><div class="toggle-line"><div><strong>同格式压缩</strong><p>保持格式，仅在文件变小时保存压缩结果。</p></div><button class="switch" role="switch" aria-checked="false" aria-label="同格式压缩"></button></div><div class="toggle-line"><div><strong>转换为 WebP</strong><p>将 JPEG 和 PNG 转换为更轻量的 WebP 图片。</p></div><button class="switch" role="switch" aria-checked="false" aria-label="转换为 WebP"></button></div><button class="button" data-demo-save>保存处理设置</button></section><section class="setting-section" id="security"><h2>账号安全</h2><p>更新密码，保护你的图片空间。</p><div class="settings-fields" style="margin-top:22px"><label>当前密码<input class="field" type="password" placeholder="输入当前密码"></label><label>新密码<input class="field" type="password" placeholder="至少 12 个字符"></label></div><button class="button" data-demo-save>修改密码</button></section></div></div>`
  main.querySelectorAll('.switch').forEach(button => button.addEventListener('click', () => { button.classList.toggle('on'); button.setAttribute('aria-checked', button.classList.contains('on') ? 'true' : 'false') }))
  main.querySelectorAll('[data-demo-save]').forEach(button => button.addEventListener('click', () => notify('设计原型 · 没有修改实际设置')))
  main.querySelectorAll('[data-section]').forEach(button => button.addEventListener('click', () => { main.querySelectorAll('[data-section]').forEach(item => item.classList.toggle('active', item === button)); main.querySelector(`#${button.dataset.section}`).scrollIntoView({ behavior: reducedMotion() ? 'instant' : 'smooth', block: 'start' }) }))
}

function renderSupporting() {
  if (page === 'migration') {
    main.innerHTML = `${heading('迁移文件', '旧地址不变，图片依然触手可及。', `<button class="button" data-demo="示例：重新扫描迁移目录">${icon('folder')}刷新文件</button>`, '整理已有图片')}<div class="status-notice">${icon('lock')}<div><strong>迁移目录为只读模式</strong><p>可以浏览、打开与复制原始链接。文件将留在原来的位置。</p></div><span>只读</span></div><div class="library-toolbar"><div class="library-tabs"><button class="active">全部迁移文件</button></div><div class="library-tools"><button class="button filter-button" data-demo="示例：展开文件名、路径、格式筛选">${icon('filter')}筛选</button><div class="view-toggle"><button class="active" aria-label="网格">${icon('grid')}</button><button aria-label="列表" data-demo="示例：切换列表展示">${icon('list')}</button></div></div></div><div class="grid">${images.slice(0,3).map((image,i)=>`<article class="image-tile" style="--i:${i}"><button class="photo-link" data-demo="示例：打开原始迁移路径" aria-label="打开 ${image.name}"><img src="assets/sample-${image.id}.jpg" alt="${image.name}"></button><div class="image-meta"><div><h2>${image.name}</h2><p class="path-text">/i/2026/08/${image.id}.jpg</p><p>${image.size} · JPEG</p></div><button class="tile-menu" data-demo="示例：复制原始路径链接" aria-label="复制链接">${icon('copy')}</button></div></article>`).join('')}</div><div class="library-end">3 个迁移文件 · 原路径保留</div>`
  } else if (page === 'aliases') {
    main.innerHTML = `${heading('路径映射', '让熟悉的链接，继续指向正确的图片。', '', '连接旧地址与新图片')}<div class="support-columns"><section class="section-panel"><div class="panel-heading"><span class="panel-icon">${icon('link')}</span><div><h2>创建映射</h2><p>上传一张图片，为它关联一个历史路径。</p></div></div><form class="study-form"><label>历史路径<input class="field" placeholder="/images/my-photo.jpg"></label><label>图片文件<button class="file-choice" type="button" data-demo="示例：选择本地图片">${icon('upload')}选择图片文件 <small>JPEG、PNG、WebP 或 GIF</small></button></label><label>来源标记<input class="field" value="admin"></label><button class="button primary" type="submit">${icon('plus')}创建映射</button></form></section><section class="section-panel resolve-panel"><div class="panel-heading"><span class="panel-icon">${icon('search')}</span><div><h2>解析路径</h2><p>查看一个旧地址现在指向哪里。</p></div></div><form class="study-form"><label>查询历史路径<input class="field" placeholder="/images/my-photo.jpg"></label><button class="button" type="submit">查找映射 ${icon('arrow')}</button></form><div class="resolve-hint">${icon('link')}<p>映射后的旧链接会直接返回图片。<br>你不需要逐一修改已发布的文章。</p></div></section></div><section class="existing-section"><div class="queue-heading"><h2>现有映射 <small>· 2</small></h2><span class="section-label">原始路径 → 图片</span></div>${images.slice(0,2).map(image=>`<article class="mapping-row"><img src="assets/sample-${image.id}.jpg" alt=""><div><code>/blog/2026/${image.id}.jpg</code><p>${image.name} <span>· 管理后台</span></p></div><button class="icon-button" data-demo="示例：复制映射链接" aria-label="复制映射链接">${icon('copy')}</button><button class="icon-button" data-demo="示例：打开映射操作菜单，含删除确认" aria-label="映射操作">${icon('more')}</button></article>`).join('')}</section>`
  } else if (page === 'tokens') {
    main.innerHTML = `${heading('API 令牌', '让你的应用，也能使用这份图片空间。', '', '访问与授权')}<div class="support-columns token-columns"><section class="section-panel"><div class="panel-heading"><span class="panel-icon">${icon('key')}</span><div><h2>创建令牌</h2><p>只授予应用需要的权限。</p></div></div><form class="study-form"><label>令牌名称<input class="field" placeholder="例如：博客发布工具"></label><fieldset><legend>权限范围</legend>${[['images:upload','上传图片'],['images:read_private','读取私密图片'],['images:delete','删除图片'],['aliases:write','写入路径映射']].map(([scope,label])=>`<label class="scope-choice"><input type="checkbox"><span><strong>${label}</strong><code>${scope}</code></span></label>`).join('')}</fieldset><label>过期时间 <span class="field-optional">可选，留空则不自动过期</span><input class="field" type="date"></label><button class="button primary" type="submit">${icon('plus')}创建令牌</button></form></section><section class="token-list"><div class="queue-heading"><h2>现有令牌 <small>· 2</small></h2></div>${[['博客发布工具','images:upload','有效','2026 / 12 / 31'],['自动化测试','images:upload · images:delete','已吊销','永不过期']].map(([title,scope,status,expires],i)=>`<article class="token-item"><div class="token-item-title"><span class="token-icon">${icon('key')}</span><h3>${title}</h3><span class="status-pill ${i?'muted-status':''}">${status}</span></div><p class="token-prefix">is_••••••••••••</p><div class="scope-tags">${scope.split(' · ').map(s=>`<code>${s}</code>`).join('')}</div><div class="token-item-footer"><span>${expires}</span>${i?'':`<button class="button text" data-demo="示例：吊销前显示确认对话框">吊销</button>`}</div></article>`).join('')}<div class="security-note">${icon('lock')}<p>令牌明文只会在创建后显示一次。<br>ImageSilo 不会保存它。</p></div></section></div>`
  } else {
    main.innerHTML = `${heading('系统状态', '图片、存储与运行情况，尽在掌握。', `<button class="button" data-demo="示例：开始系统巡检">${icon('activity')}立即巡检</button>`, '空间运行概览')}<div class="metric-grid">${[['图片数量','6','张图片','image'],['存储用量','10.8','MB','folder'],['迁移文件','5.3','MB · 保留原路径','link']].map(([label,value,unit,name])=>`<section class="metric-panel"><div>${label}${icon(name)}</div><p>${value}<small>${unit}</small></p></section>`).join('')}</div><div class="system-columns"><section class="section-panel"><div class="panel-heading"><div><h2>运行资源</h2><p>当前实例的内存与并发状态。</p></div><span class="status-pill">运行中</span></div><dl class="resource-list"><dt>进程内存 RSS</dt><dd>42.6 MB</dd><dt>Go Heap</dt><dd>8.4 MB</dd><dt>Go Heap Sys</dt><dd>16.0 MB</dd><dt>Goroutine</dt><dd>12</dd><dt>历史路径映射</dt><dd>2</dd></dl></section><section class="section-panel"><div class="panel-heading"><div><h2>数据与索引</h2><p>数据库与内存中的记录保持一致。</p></div></div><div class="index-health"><span>${icon('check')}</span><div><strong>一切正常</strong><p>图片与历史路径均可正常访问。</p></div></div><div class="index-counts"><span>图片 <b>6</b></span><span>映射 <b>2</b></span><span>Session <b>1</b></span><span>Token <b>1</b></span></div><button class="button" data-demo="示例：确认后重建内存缓存">重建缓存</button></section></div><section class="section-panel maintenance-panel"><div class="panel-heading"><div><h2>最近维护</h2><p>必要时可以手动巡检，日常维护自动完成。</p></div></div><div class="maintenance-list">${[['最近巡检','今天 14:30','未发现缺失文件、孤立图片或临时文件。'],['最近重建','今天 09:12','已加载 6 张图片与 2 条历史路径。'],['每日维护','今天 03:00','临时文件与孤立缩略图清理完成。']].map(([title,time,description])=>`<div><span class="timeline-dot"></span><div><h3>${title}<time>${time}</time></h3><p>${description}</p></div>${icon('check')}</div>`).join('')}</div></section><p class="prototype-footnote">以上为设计示例。正式界面仅呈现后端返回的实际数值。</p>`
  }
  main.querySelectorAll('[data-demo]').forEach(button=>button.addEventListener('click',()=>notify(button.dataset.demo)))
  main.querySelectorAll('form').forEach(form=>form.addEventListener('submit',event=>{event.preventDefault();notify('设计原型 · 未执行实际写入')}))
}

function render() {
  if (page === 'library') renderLibrary()
  else if (page === 'upload') renderUpload()
  else if (page === 'detail') renderDetail()
  else if (page === 'settings') renderSettings()
  else renderSupporting()
  document.querySelector('#breadcrumb').textContent = names[page]
  document.querySelectorAll('[data-page]').forEach(button => button.classList.toggle('active', button.dataset.page === (page === 'detail' ? 'library' : page)))
  updateBatch()
}

function go(next) {
  if (next === page) return
  page = next
  closePrototypeMenu(false)
  document.querySelector('.sidebar').classList.remove('mobile-open')
  transition(render)
  window.scrollTo({ top: 0, behavior: 'instant' })
}

function bindNavigation() { main.querySelectorAll('[data-go]').forEach(button => button.addEventListener('click', () => go(button.dataset.go))) }
function updateBatch() { document.querySelector('#batch').hidden = selected.size === 0 || page !== 'library'; document.querySelector('#selected-count').textContent = selected.size }
function notify(message) { clearTimeout(toastTimer); const toast = document.querySelector('#toast'); toast.querySelector('span').textContent = message; toast.classList.add('visible'); toastTimer = setTimeout(() => toast.classList.remove('visible'), 2200) }
function closePrototypeMenu(restoreFocus=true) {
  if(!openMenuElement) return
  const closing = openMenuElement
  openMenuElement = null
  closing.inert = true
  closing.classList.add('is-closing')
  if(restoreFocus) menuAnchor?.focus()
  setTimeout(()=>closing.remove(),reducedMotion()?0:140)
}
function openPrototypeMenu(anchor,items) {
  closePrototypeMenu(false)
  menuAnchor=anchor
  const menu=document.createElement('div')
  menu.className='prototype-menu'
  menu.setAttribute('role','menu')
  menu.innerHTML=items.map((item,i)=>`<button role="menuitem" data-item="${i}" class="${item.destructive?'destructive':''}"><span>${item.label}</span>${item.selected?icon('check'):''}</button>`).join('')
  document.body.append(menu)
  const rect=anchor.getBoundingClientRect(),height=menu.offsetHeight
  menu.style.left=`${Math.max(12,Math.min(innerWidth-212,rect.right-200))}px`
  const dock=anchor.closest('.batch-dock')
  const dockTop=dock?innerHeight-parseFloat(getComputedStyle(dock).bottom)-dock.offsetHeight:null
  menu.style.top=`${dockTop!==null?Math.max(12,dockTop-height-8):rect.bottom+height+16>innerHeight?Math.max(12,rect.top-height-8):rect.bottom+8}px`
  openMenuElement=menu
  menu.querySelectorAll('button').forEach((button,i)=>button.addEventListener('click',()=>{items[i].action();closePrototypeMenu()}))
  menu.addEventListener('keydown',event=>{
    if(event.key==='Escape'){event.stopPropagation();closePrototypeMenu();return}
    if(event.key==='Tab'){closePrototypeMenu(false);return}
    if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return
    event.preventDefault()
    const buttons=[...menu.querySelectorAll('button')],current=buttons.indexOf(document.activeElement)
    const next=event.key==='Home'?0:event.key==='End'?buttons.length-1:(current+(event.key==='ArrowDown'?1:-1)+buttons.length)%buttons.length
    buttons[next].focus()
  })
  menu.querySelector('button')?.focus()
}
document.addEventListener('pointerdown',event=>{if(openMenuElement&&!openMenuElement.contains(event.target)&&!menuAnchor?.contains(event.target))closePrototypeMenu(false)})
document.querySelectorAll('[data-page]').forEach(button => { button.setAttribute('aria-label', names[button.dataset.page]); button.addEventListener('click', () => go(button.dataset.page)) })
document.querySelector('.brand').addEventListener('click', event => { event.preventDefault(); go('library') })
document.querySelectorAll('[data-direction]').forEach(button => button.addEventListener('click', () => {
  direction = button.dataset.direction
  transition(() => { document.body.dataset.layout = direction; document.body.classList.toggle('dark', direction === 'darkroom'); document.querySelectorAll('[data-direction]').forEach(item => item.classList.toggle('active', item === button)) })
}))
document.querySelector('#theme-toggle').addEventListener('click', () => document.body.classList.toggle('dark'))
document.querySelector('#clear-selection').addEventListener('click', () => { selected.clear(); renderLibrary(); updateBatch() })
document.querySelector('#batch-copy').addEventListener('click', () => notify(`示例：已复制 ${selected.size} 张图片的链接`))
document.querySelector('#batch-format').addEventListener('click',event=>openPrototypeMenu(event.currentTarget,['直链','Markdown','BBCode','HTML'].map(format=>({label:format,selected:format===linkFormat,action:()=>{linkFormat=format;document.querySelector('#batch-copy').innerHTML=icon('copy')+'复制'+format}}))))
document.querySelector('#batch-privacy').addEventListener('click', event => openPrototypeMenu(event.currentTarget,[{label:'改为公开',action:()=>notify(`示例：${selected.size} 张图片改为公开`)},{label:'改为私密',action:()=>notify(`示例：${selected.size} 张图片改为私密`)}]))
document.querySelector('.dock-more').addEventListener('click', event => openPrototypeMenu(event.currentTarget,[{label:'删除选中图片…',destructive:true,action:()=>notify('示例：正式删除前会显示二次确认')}]))
document.querySelector('#global-search').addEventListener('click', () => { filtersOpen = true; if (page !== 'library') go('library'); else renderLibrary(); requestAnimationFrame(() => main.querySelector('input[name="search"]')?.focus()) })
document.querySelector('.mobile-menu').addEventListener('click', () => document.querySelector('.sidebar').classList.toggle('mobile-open'))
document.querySelector('#review-notes').addEventListener('click', () => document.querySelector('#notes-dialog').showModal())
document.querySelector('.dialog-close').addEventListener('click', () => document.querySelector('#notes-dialog').close())
document.addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); document.querySelector('#global-search').click() } if (event.key === 'Escape') document.querySelector('.sidebar').classList.remove('mobile-open') })
render()
