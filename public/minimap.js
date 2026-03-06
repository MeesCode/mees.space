
class Minimap {

    constructor(element, scale) {
        this.element = element
        this.scale = scale
        this._dragging = false
        this._lastClientY = 0
    }

    init() {
        document.documentElement.style.setProperty('--minimap-scale', this.scale)

        document.addEventListener('scroll', _ => this.scrollHandler())
        window.addEventListener('resize', _ => this.resizeHandler())

        // create map
        this.map = document.createElement('div')
        this.map.classList.add('minimap')
        this.map.innerHTML = this.element.innerHTML
        document.body.appendChild(this.map)

        // create overlay
        this.overlay = document.createElement('div')
        this.overlay.classList.add('minimap-overlay')
        this.overlay.addEventListener('click', event => this.clickHandler(event))
        document.body.appendChild(this.overlay)

        // create viewport
        this.viewport = document.createElement('div')
        this.viewport.classList.add('minimap-viewport')
        this.viewport.addEventListener('mousedown', event => this.startDrag(event))
        document.addEventListener('mousemove', event => this.dragHandler(event))
        document.addEventListener('mouseup', _ => this.stopDrag())
        document.body.appendChild(this.viewport)

        // set initial values
        this.resizeHandler()
        this.scrollHandler()
    }

    setReflowInterval(millis) {
        clearInterval(this.interval)
        this.interval = setInterval(() => {
            this.scrollHandler()
            this.resizeHandler()
        }, millis)
    }

    startDrag(event) {
        event.preventDefault()
        this._dragging = true
        this._lastClientY = event.clientY
        this.viewport.classList.add('dragging')
    }

    stopDrag() {
        this._dragging = false
        this.viewport.classList.remove('dragging')
    }

    resizeHandler() {
        let contentEl = this.element.querySelector('#content')
        this.width = contentEl ? contentEl.clientWidth : this.element.clientWidth
        document.documentElement.style.setProperty('--minimap-width', `${this.width}px`)

        this.overlay.style.height = `${this.map.scrollHeight * this.scale}px`

        this.viewportheight = (window.innerHeight / this.element.scrollHeight) * (this.map.scrollHeight * this.scale)
        this.viewport.style.height = `${this.viewportheight}px`
    }

    scrollHandler() {
        let max_scroll = this.element.scrollHeight - window.innerHeight
        let percentage = window.scrollY / max_scroll

        let top = -0.5 * (this.map.clientHeight - (this.map.clientHeight * this.scale))
        let mapheight = this.map.scrollHeight * this.scale

        let offset = 0
        if (mapheight > window.innerHeight) {
            offset = percentage * (mapheight - window.innerHeight)
        }

        this.map.style.top = `${top - offset}px`
        this.overlay.style.top = `${-offset}px`

        let scrollheight = Math.min(this.map.scrollHeight * this.scale, window.innerHeight)
        this.viewport.style.top = `${percentage * (scrollheight - this.viewportheight)}px`
    }

    dragHandler(event) {
        if (!this._dragging) return

        let delta = event.clientY - this._lastClientY
        this._lastClientY = event.clientY
        window.scrollTo(0, window.scrollY + delta * (1 / this.scale))
    }

    clickHandler(event) {
        window.scrollTo(0, (event.offsetY - this.viewport.clientHeight / 2) * (1 / this.scale))
    }

}
