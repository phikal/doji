"use strict";
const moffset = 0.38196601125; // maximal playback offset (sec)
const rwait = 50;    // ms wait for website to react
const scint = 20000; // ms to wait between status requests
const bufwait = 500; // ms to wait between buffering checks
const lwait = 100;   // ms to wait before rechecking a lock

function getElementById(id) {
	var elem = document.getElementById(id);
	if (elem == null)
		throw "non-existent element \"" + id + "\"";
	return elem;
}

const video = getElementById("video");
const users = getElementById("usr");
const prompt = getElementById("prompt");
const log = getElementById("msg");
const videos = getElementById("videos");
const queue = getElementById("queue");
const sets = getElementById("sets");
const requests = getElementById("reqs");

function getCookie(name) {
	var value = "; " + document.cookie;
	var parts = value.split("; " + name + "=");
	if (parts.length == 2) return parts.pop().split(";").shift();
}

const user = document.cookie.replace(/(?:(?:^|.*;\s*)user\s*\=\s*([^;]*).*$)|^.*$/, "$1") ||
      window.prompt("Username", (Math.random() + 1).toString(36).substring(7));
document.cookie = "user=" + user + ";path=/";

const conf = JSON.parse(getCookie("conf") || "{}");

const menus = [ "videos", "queue", "sets", "settings", "help", "info" ];
function choose_menu(id) {
	document.cookie = "tab=" + id;
	menus.map(m => {
		getElementById(m).style.display = (m === id) ? "" : "none";
		if (m === id)
			getElementById(m + "-b").classList.add("choice");
		else
			getElementById(m + "-b").classList.remove("choice");
	});
}

menus.map(m => getElementById(m + "-b").onclick = () => choose_menu(m));
choose_menu(getCookie("tab") || "videos");

function format_time(sec) {
	var hour = Math.floor(sec / 3600),
        min  = ("0" + Math.floor(sec / 60)).slice(-2),
        sec  = ("0" + (sec % 60)).slice(-2);
	return (hour == 0 ? "" : hour + ":") + min + ":" + sec;
}

function jump_to(sec, play) {
    if (sec === undefined || sec < 0)
        return;

    function jump() {
        if (Math.abs(sec - video.currentTime) > moffset)
			video.currentTime = sec;

		if (video.readystate > 2) {
			if (play && video.playing)
				video.play();
			else video.pause();
		}
    }

    function check() {
        var buf = video.seekable;
        for (var i = 0; i < buf.length; i++) {
            if (buf.start(i) < sec && buf.end(i) > sec) {
				jump(sec);
			} else setTimeout(check, bufwait);
        }
    }

	lock(check);
}

function load_video(vid) {
	if (!vid) return;
    var parser = document.createElement('a');
    parser.href = video.src;
	video.controls = true;
	var ovid = vid;
    vid = "./d" + window.location.pathname + "/" + vid;
    if (vid != "." + parser.pathname) { // TODO: make cleverer
        video.src = vid;
        video.load();
        video.currentTime = 0;
		getElementById("title").innerText = ovid;
		video.pause();
		remote = false;
    }
}

var sfilter = false;
function exec_cmd(cmd) {
    var matches = cmd.match(/^\/(\w+)(?:\s+(.+)\s*)?/);
    if (!matches)
        return false;
    var arg = "";
    if (matches.length > 2)
        arg = matches[2];
    switch (matches[1]) {
    case "volume":
        if (arg) {
            video.volume = arg;
            log_msg("event", {
                msg: "<em>set volume to " + (arg * 100) + "%</em>"
            });
        }
        break;
	case "search":
		sfilter = arg || new RegExp(arg, "i");
	case "status":
		send("status");
		break;
	default:
		return false;
    }
    return true;
}

var user_map;
function get_name(id) {
	var user = user_map[id];
	return user && user.name;
}

function load_status(data) {
    while(videos.firstChild)
        videos.removeChild(videos.firstChild);
    if (data.vids)
        data.vids.map(v, i => {
            var vid = document.createElement("li");
            vid.appendChild(document.createTextNode(v));
            vid.onclick = _ => send(conf.queue ? "push" : "select", {
                val: i
            });
            videos.appendChild(vid);
        });

	if (data.queue) {
		while(queue.firstChild)
			queue.removeChild(queue.firstChild);
		data.queue.map((d, i) => {
			var que = document.createElement("li");
            que.appendChild(document.createTextNode(d));
            que.onclick = _ => send("pop", { val: i });
            queue.appendChild(que);
		})
	}

	if (data.sets) {
		while(sets.firstChild)
			sets.removeChild(sets.firstChild);
        Object.keys(data.sets).sort().map(s => {
			if (sfilter && !s.match(sfilter))
				return;

            var vid = document.createElement("li");
            vid.appendChild(document.createTextNode(s));
			if (s in data.lsets)
				vid.classList.add("loaded");
            vid.onclick = _ => send("load", { msg: s });
            sets.appendChild(vid);
        });
	}

    while(users.firstChild)
        users.removeChild(users.firstChild);
	user_map = data.users;
    for (var u in data.users) {
        var tag = document.createElement("li");
        tag.appendChild(document.createTextNode(u.name));
        if (u == user)
            tag.classList.add("choice");
        users.appendChild(tag);
    }

	if (data.reqs) {
		while(reqs.firstChild)
			reqs.removeChild(reqs.firstChild);
		data.reqs.map(r => {
			var canvas = document.createElement("canvas");
			var ctx = canvas.getContext("2d");
			ctx.fillStyle = "#da8";
			ctx.fillRect(0, 0, canvas.width * r.progress, canvas.height);
			ctx.fillStyle = "#000";
			ctx.font = "14px monospace";
			ctx.fillText(r.url, 2, 2);
			reqs.appendChild(canvas);
		});
	}

	lock(_ => {
		load_video(data.playing);
		jump_to(data.progress, true);

		if (video.paused && !data.paused)
			video.play();
		else if (!video.paused && data.paused)
			video.pause();
	});
}

var lastmsg;
function log_msg(type_or_msg, msg) {
	if (!msg) {
		msg = type_or_msg;
		if (!msg.type)
			throw "invalid message";
	} else {
		if (type_or_msg)
			msg.type = type_or_msg;
	}

    var l = document.createElement("li");
    l.classList.add(msg.type);
    switch(msg.type) {
	case "mute":
		video.volume = 1 - msg.val;
		break;
    case "talk":
        l.innerHTML =  "<b>" + get_name(msg.from) + ":</b> " +
			msg.msg;
        break;
    case "pause":
        l.innerHTML = "<b>" + get_name(msg.from) + "</b> " +
            "paused the video";
        break;
    case "play":
        l.innerHTML = "<b>" + get_name(msg.from) + "</b> " +
            "started playing the video";
        break;
    case "seek":
		if (msg.val)
			l.innerHTML = "<b>" + get_name(msg.from) + "</b> "+
			"moved to " +
			format_time(msg.val);
        break;
    case "select":
        l.innerHTML = "<b>" + get_name(msg.from) + "</b> " +
			"selected new video: <code>" +
            msg.msg + "</code>";
        break;
	case "push":
		l.innerHTML = "<b>" + get_name(msg.from) + "</b> " +
			"added new video to queue: <code>" +
            msg.msg + "</code>";
		break;
	case "load":
		l.innerHTML = "<b>" + get_name(msg.from) + "</b> " +
			(msg.data ? "unloaded" : "loaded") +
			" new set: <code>" +
            msg.msg + "</code>";
		break;
    case "event":
    case "request":
        l.innerHTML = msg.msg;
        break;
    default:
        return;
    }
    log.appendChild(l);
    log.scrollTop = log.scrollHeight;
}
var remote = false; /* hacked asf */
const socket = new WebSocket(location.href.replace(/http/, "ws") +
                             "/socket");

function lock(func) {
	if (!func)
		throw "locked with empty function";

	if (conf.unlock) {
		func();
		return;
	}

	if (remote) {
		setTimeout(_ => lock(func), lwait);
	} else {
		remote = true;
		func();
		setTimeout(_ => remote = false, rwait);
	}
}

function send(type, msg) {
	console.log(type + " :: " + JSON.stringify(msg));
	msg = msg || {};
	msg.type = type;
	lock(_ => {
		if (socket.readyState < 1)
			setTimeout(0.1, _ => send(msg));
		else
			socket.send(JSON.stringify(msg));
	});
}

const save = _ => document.cookie = "conf=" +
	  JSON.stringify(conf) +
	  ";  path=/";
const setbody = getElementById("setbody");
function addconf(name, type, run) {
	var row = document.createElement("tr");

	var lab = document.createElement("td");
	lab.innerText = name;
	row.appendChild(lab);

	var opt = document.createElement("td");
	var input = document.createElement("input");
	input.type = type;
	opt.appendChild(input);
	row.appendChild(opt);

	setbody.appendChild(row);

	run = run || (_ => 0);
	name = name.replace(/[^a-z]/gi, "").toLowerCase();
	switch (type) {
	case "checkbox":
		input.onclick = _ => {
			conf[name] = input.checked;
			save();
			run();
		};
		input.checked = conf[name];
		break;
	default:
		input.onclick = _ => {
			conf[name] = input.value
			save();
			run();
		};
		input.value = conf[name];
		break;
	}
}

addconf("Queue?", "checkbox");
addconf("Unhook", "checkbox");

// events
socket.onmessage = e => {
	if (conf.unhook)
		return true;

	console.log(e.data);
	var msg = JSON.parse(e.data);
    var timestamp = new Date().toLocaleTimeString();
	lock(_ => {
		switch(msg.type) {
		case "pause":
			if (msg.from != user)
				jump_to(msg.val, false);
			if (msg.msg)
				log_msg("event", { msg: "Paused for " + msg.msg });
			break;
		case "play":
			if (video.paused)
				jump_to(msg.val, true);
			break;
		case "seek":
			if (lastmsg.type != "seek" && msg.from != user)
				jump_to(msg.val, true);
			break;
		case "select":
			video.pause();
			load_video(msg.msg);
			break;
		case "load":
			for (var i = 0; i < sets.children.length; i++) {
				if (sets.children[i].innerText == msg.msg) {
					sets.children[i].classList.toggle("loaded");
					break;
				}
			}
			break;
		case "status":
			load_status(msg.data);
			break;
		}
		if (msg)
			lastmsg = msg;
		log_msg(msg);
	});
};

// input events
prompt.onkeypress = e => {
	if (e.key == "Enter" && prompt.value) {
        if (!exec_cmd(prompt.value))
            send("talk", { msg: prompt.value.trim() });
        prompt.value = "";
    }
};

// window events
window.onbeforeunload = _ => socket.close();

// setup
socket.onerror = err => log_msg("event", {
	msg: "<b>socket error:</b> <code>" + err + "</code>!"
});
socket.onclose = e => {
	if (false) {
		log_msg("event", {
			msg: "username already taken; reload page"
		});
		document.cookie = "";
		remote = true;
	} else log_msg("event", {
		msg: "<em>socket" + (e.wasClean ? "" : " abruptly") +
			" closed with code " + e.code + "</em>"
	});
}

// request data to load
socket.onopen = e => {
    var name = window.location.pathname.substring(1);
    var check = _ => send("status");
    log_msg("event", {
        msg: "connected to <b>" + name + "</b> as <b>" + user + "</b>!"
    })

	// video events
	video.onpause = e => send("pause", { val: video.currentTime });
	video.onplay = e => send("play", { val: video.currentTime });
	video.onseeking = e => send("seek", { val: video.currentTime });
	video.onended = _ => {
		video.src = "";
		video.controls = false;
		setTimeout(_ => send("next"), bufwait);
	};
	video.oncanplay = _ => {
		if (video.paused || !conf.unhook)
			send("ready");
	};
	video.onstalled = _ => send("pause", { msg: user })

	check();
    setInterval(check, scint);
};
