
class RFB {
	constructor(width, height) {
		this.width = width;
		this.height = height;
		this.tmax = 0.0;
		this.currentTime = 0.0;
		this.playing = false;
		this.lastFrame = 0.0;
		this.pointer = {
			X: 0.0,
			Y: 0.0,
			canvas: null,
			ctx: null,
		}
		this.events = [];
	}

	PushEvent(type, time, data) {
		if ( time > this.tmax ) {
			this.tmax = time;
		}

		if ( type != "keypress" && type != "keyrelease" ) {
			this.events.push({type, time, data});
		}
	}

	Render(elt) {
		this.tmax = Math.floor( this.tmax + 250 );

		this.canvas = elt.querySelector(".-framebuffer");
		this.canvas.height = this.height;
		this.canvas.width = this.width;

		this.ctx = this.canvas.getContext("2d");

		this.pointer.canvas = elt.querySelector(".-spritelayer");
		this.pointer.canvas.height = this.height;
		this.pointer.canvas.width = this.width;
		this.pointer.ctx = this.pointer.canvas.getContext("2d");

		this.playbutton = elt.querySelector(".-vic-controls .-playpause");
		this.playbutton.innerText = "";
		this.playbutton.addEventListener("click", () => this.TogglePlaying());

		this.seekbar = elt.querySelector(".-vic-controls .-seek");
		this.seekbar.id = "seekbar_" + Math.random().toString(36).substr(2);
		this.seekbar.type = "range";
		this.seekbar.min = 0;
		this.seekbar.max = this.tmax;
		this.seekbar.step = 0.1;
		this.seekbar.value = 0;

		let seek = () => this.seek();
		this.seekbar.addEventListener("change", seek);
		this.seekbar.addEventListener("input", seek);


		this.seekbarLabel = elt.querySelector(".-vic-controls .-playtime");
		this.seekbarLabel.htmlFor = this.seekbar.id;

		this.speedknob = elt.querySelector(".-vic-controls .-speedknob");

		window.addEventListener("resize", () => this.resizeSpriteLayer());
		this.resizeSpriteLayer();

		this.Reset();
		this.Pause();
	}

	Reset() {
		this.eventIndex = 0;
		this.setTime(0);
		this.ctx.fillStyle = 'rgb( 0, 0, 0 )';
		this.ctx.fillRect( 0, 0, this.width, this.height );

		// Get rid of the pointer
		this.pointer.X = -20;
		this.pointer.Y = -20;
		this.blitMouse();
	}

	Play() {
		this.playing = true;
		this.playbutton.innerText = "\u258c\u258c";

		window.requestAnimationFrame( (time) => {
			this.lastFrame = time;
			window.requestAnimationFrame( (t) => this.nextFrame(t) );
		} );
	}

	Pause() {
		this.playing = false;
		this.playbutton.innerText = "\u25b6";
	}

	TogglePlaying() {
		if ( this.playing ) {
			this.Pause();
		} else {
			this.Play();
		}
	}

	nextFrame( time ) {
		if ( !this.playing ) {
			return;
		}
		let tnew = this.currentTime + ( time - this.lastFrame ) * parseFloat(this.speedknob.value);
		if ( tnew > this.tmax ) {
			tnew = this.tmax;
			this.Pause();
		}
		this.setTime(tnew);

		this.lastFrame = time;
		window.requestAnimationFrame( (t) => this.nextFrame(t) );
	}

	seek() {
		this.setTime(parseFloat(this.seekbar.value));
	}

	setTime( time ) {
		this.currentTime = time;

		let i, n = 0;
		for ( i = 0; i < this.events.length; i++ ) {
			if ( this.events[i].time < time ) {
				n = i+1;
			}
		}
		this.setEventIndex(n);

		let t = time / 1000;
		let m = Math.floor( t / 60 );
		let s = t - m;
		let z = (s) => s < 10 ? "0" : "";
		this.seekbarLabel.innerText = "" + z(m) + m + ":" + z(s) + s.toFixed(1);

		if ( this.seekbar.value != time ) {
			this.seekbar.value = time;
		}
	}

	setEventIndex(idx) {
		if ( this.eventIndex > idx ) {
			this.Reset();
		}
		for ( let i = this.eventIndex; i < idx; i++ ) {
			this.applyEvent(i);
		}
		if ( idx > this.eventIndex ) {
			this.blitMouse();
		}
		this.eventIndex = idx;

		this.seekbarLabel.innerText = "event " + idx;

		if ( this.seekbar.value != idx ) {
			this.seekbar.value = idx;
		}
	}

	applyEvent(idx) {
		let event = this.events[idx];
		if ( event.type == "framebuffer" ) {
			this.applyFramebuffer(event.data);
		} else if ( event.type == "pointerupdate" ) {
			this.applyPointerUpdate(event.data);
		}
	}

	applyFramebuffer(fbdata) {
		let img = document.getElementById(fbdata.Id);
		if ( img ) {
			this.ctx.drawImage(img, 0, 0);
		}
	}

	applyPointerUpdate(pdata) {
		this.pointer.X = pdata.X;
		this.pointer.Y = pdata.Y;
	}

	resizeSpriteLayer() {
		let rect = this.canvas.getBoundingClientRect();
		this.pointer.canvas.style.width = rect.width + "px";
		this.pointer.canvas.style.height = rect.height + "px";
	}

	blitMouse() {
		this.pointer.ctx.clearRect(0, 0, this.width, this.height);

		this.pointer.ctx.fillStyle = 'rgba( 255, 30, 30, 0.7 )';
		this.pointer.ctx.beginPath();
		this.pointer.ctx.ellipse(this.pointer.X, this.pointer.Y, 3, 3, 0, 0, Math.PI*2);
		this.pointer.ctx.fill();
	}
}

