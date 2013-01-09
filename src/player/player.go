// player package is the core of the program: it manages playlists and player's state.
package player

import (
	"../audio"
	"container/list"
	"fmt"
)

// Playing process communication command response.
type response struct {
	// Error.
	err error
	// Command processing result itself.
	arguments interface{}
}

// Playing process communication command.
type command struct {
	code         int
	arguments    []interface{}
	responseChan chan *response
}

// Player engine object.
type Player struct {
	// All available output drivers.
	outputs []audio.Output
	// All available audio decoders.
	decoders []audio.Decoder
	// All (user and system) playlists list.
	playlists *list.List
	// Channel to communicate player with. Client code can
	// write commands and read responses to/from the channel.
	playingChan chan *command
}

// New returns a newly created Player object.
func New() *Player {
	p := new(Player)
	p.playlists = list.New()
	p.playingChan = make(chan *command, 10)

	return p
}

// Register new available output driver.
func (player *Player) RegisterOutput(o audio.Output) {
	player.outputs = append(player.outputs, o)
}

// Register new audio decoder driver.
func (player *Player) RegisterDecoder(d audio.Decoder) {
	player.decoders = append(player.decoders, d)
}

// Run starts Player execution.
func (player *Player) Run() {
	go player.playingProcess()
}

// Command allows to communicate with Player by sending him commands.
func (player *Player) Command(cmd int, args ...interface{}) (res interface{}, err error) {
	c := &command{code: cmd, arguments: args, responseChan: make(chan *response, 1)}

	player.playingChan <- c
	resp := <-c.responseChan

	return resp.arguments, resp.err
}

// playingProcess is the core of the player. It runs in goroutine
// and does the playing intself. Outer world can affects to playing process by
// sending commands via playingChan of the Player struct.
func (player *Player) playingProcess() {
	for {
		cmd := <-player.playingChan

		r := new(response)

		switch cmd.code {
		case CMD_PLAYLISTS_LIST:
			r.arguments = player.cmdPlaylistsList()
		case CMD_PLAYLISTS_ADD:
			player.cmdPlaylistsAdd(cmd.arguments[0].(string))
		case CMD_PLAYLISTS_DELETE:
			r.err = player.cmdPlaylistsDelete(cmd.arguments[0].(string))
		default:
			r.err = fmt.Errorf("Unsupported command %s.", cmd.code)
		}

		cmd.responseChan <- r
	}
}

// Returns playlists list.
func (player *Player) cmdPlaylistsList() []*Playlist {
	playlists := make([]*Playlist, player.playlists.Len())

	i := 0
	for e := player.playlists.Front(); e != nil; e = e.Next() {
		playlists[i] = e.Value.(*Playlist)
		i++
	}

	return playlists
}

// Creates new empty playlist with give name. Playlist name should be unique,
// so if playlist with given name exists error will be returned. 
func (player *Player) cmdPlaylistsAdd(name string) error {
	if player.getPlaylistByName(name) != nil {
		return fmt.Errorf("Playlist %s already exists.", name)
	}

	player.playlists.PushBack(newPlaylist(name))

	return nil
}

// Deletes existing playlist by name.
func (player *Player) cmdPlaylistsDelete(name string) error {
	// TODO: Stop playing if playing current playlist.

	for e := player.playlists.Front(); e != nil; e = e.Next() {
		playlist := e.Value.(*Playlist)
	 	if playlist.system {
	 		return fmt.Errorf("System playlist can't be deleted")
	 	}
		if e.Value.(*Playlist).Name == name {
			player.playlists.Remove(e)
			break
		}
	}

	return nil
}

// getPlaylistByName returns playlist for given name
// or nil if there is no such playlist exists.
func (player *Player) getPlaylistByName(name string) *Playlist {
	for e := player.playlists.Front(); e != nil; e = e.Next() {
		playlist := e.Value.(*Playlist)

		if playlist.Name == name {
			return playlist
		}
	}

	return nil
}

// --------------------

// Player mutex. All public player commands should be protected with this mutex lock.
// var mutex sync.Mutex

// thread is the main player thread (goroutine wrapper).
// var thread *playingThread

// // Playlist returns playlist object by name.
// func Playlist(name string) (playlist *playlist.Playlist, err os.Error) {
// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	playlist, err = getPlaylistByName(name)

// 	return
// }

// // Play start playing existing a track form an existing playlist.
// func Play(playlistName string, trackNumber int) os.Error {
// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	pl, err := getPlaylistByName(playlistName)
// 	if err != nil {
// 		return err
// 	}

// 	if trackNumber < 0 || trackNumber >= pl.Len() {
// 		return os.NewError(fmt.Sprintf("Playlist '%s' has no track number %d.",
// 			playlistName, trackNumber))
// 	}

// 	thread.Play(pl.Track(trackNumber))

// 	return nil
// }

// // Pause pause or unpause playing process.
// func Pause() {
// 	thread.Pause()
// }

// // Stop closes playing processes, frees resources.
// // This function should be called before exiting program.
// func Stop() {
// 	thread.Stop()
// }

// // Package init function.
// func init() {
// 	// Audio tagreaders.
// 	audio.RegisterTagReaderFactory(ogg.NewTagReader)
// 	// Audio outputs.
// 	audio.RegisterOutput(alsa.DriverName, alsa.New)
// 	// Audio decoders.
// 	audio.RegisterDecoderFactory(ogg.NewDecoder)

// 	// Playists
// 	playlists = make([]*playlist.Playlist, 0)
// 	// We have one system (predefined) playlist, -- *vfs*.
// 	playlists = append(playlists, playlist.New(vfs.PlaylistName))

// 	// Create and start playing thread.
// 	thread = newPlayingThread()
// 	thread.Start()
// }