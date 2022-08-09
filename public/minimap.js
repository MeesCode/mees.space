
class Minimap {

    constructor(element, scale){
        this.element = element
        this.scale = scale
    }

    init(){
        document.documentElement.style.setProperty('--minimap-scale', this.scale);

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
        this.viewport.addEventListener('mousedown', event => this.dragpostition = event.offsetY)
        this.viewport.addEventListener('mousemove', event => this.dragHandler(event))
        document.body.appendChild(this.viewport)

        // set initial values
        this.resizeHandler()
        this.scrollHandler()
    }

    resizeHandler(){
        // update width
        this.width = this.element.clientWidth
        document.documentElement.style.setProperty('--minimap-width', `${this.width}px`);

        // update overlay height
        this.overlay.style.height = `${this.map.scrollHeight}px`

        // update viewport height
        this.viewportheight = (window.innerHeight / this.element.scrollHeight) * (this.map.scrollHeight * this.scale)
        this.viewport.style.height = `${this.viewportheight}px`
    }

    scrollHandler(){
        // calc scroll percentage
        let max_scroll = this.element.scrollHeight - window.innerHeight
        let percentage = window.scrollY / max_scroll

        // set initial height and get the map height
        let top = -0.5 * (this.map.clientHeight - (this.map.clientHeight * this.scale))
        let mapheight = this.map.scrollHeight * this.scale

        // only change offset when map is larger than the screen
        let offset = 0;
        if(mapheight > window.innerHeight){
            offset = percentage * (mapheight - window.innerHeight)
        }

        // update location of map and overlay
        this.map.style.top = `${top - offset}px`
        this.overlay.style.top = `${-offset}px`

        // calc the max distance the viewport can move
        let scrollheight = Math.min(this.map.scrollHeight * this.scale, window.innerHeight)

        // update location of viewport
        this.viewport.style.top = `${percentage * (scrollheight - this.viewportheight)}px`
    }

    dragHandler(event){
        // button not held, ignore
        if(event.buttons != 1) return

        // simulate drag
        window.scrollTo(0, window.scrollY + (event.offsetY - this.dragpostition) * (1/this.scale))
    }

    clickHandler(event){
        // place the center of the viewport on the clicked location
        window.scrollTo(0, (event.offsetY - this.viewport.clientHeight/2) * (1/this.scale))
    }

}