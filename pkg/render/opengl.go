package render

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"

	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"

	"bytes"
	"encoding/base64"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() {
	// GLFW needs to run on the main thread
	runtime.LockOSThread()
}

// OpenGLCanvas implements TargetCanvas using raw hardware acceleration.
type OpenGLCanvas struct {
	window        *glfw.Window
	width, height int
	shaderProg    uint32
	vao           uint32
	vbo           uint32
	onClick       func(x, y float64)
}

const vertexShaderSource = `
#version 330 core
layout(location = 0) in vec4 vertex; // <vec2 pos, vec2 tex>
uniform mat4 projection;
uniform vec4 color;
out vec4 fragColor;
out vec2 TexCoords;
void main() {
    gl_Position = projection * vec4(vertex.xy, 0.0, 1.0);
    fragColor = color;
    TexCoords = vertex.zw;
}
` + "\x00"

const fragmentShaderSource = `
#version 330 core
in vec4 fragColor;
in vec2 TexCoords;
out vec4 outColor;

uniform sampler2D text;
uniform bool isText;

void main() {
    if (isText) {
        vec4 sampled = texture(text, TexCoords);
        outColor = vec4(fragColor.rgb, sampled.a * fragColor.a);
    } else {
        outColor = fragColor;
    }
}
` + "\x00"

// NewOpenGLCanvas creates a window and initializes OpenGL state.
func NewOpenGLCanvas(width, height int, title string) (*OpenGLCanvas, error) {
	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to init glfw: %w", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)

	window, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to init gl: %w", err)
	}

	prog, err := compileShader(vertexShaderSource, fragmentShaderSource)
	if err != nil {
		return nil, err
	}

	var vao, vbo uint32
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)

	canvas := &OpenGLCanvas{
		window:     window,
		width:      width,
		height:     height,
		shaderProg: prog,
		vao:        vao,
		vbo:        vbo,
	}

	window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		if button == glfw.MouseButtonLeft && action == glfw.Press {
			x, y := w.GetCursorPos()
			if canvas.onClick != nil {
				canvas.onClick(x, y)
			}
		}
	})

	return canvas, nil
}

// SetOnMouseClick registers a callback for mouse down events.
func (c *OpenGLCanvas) SetOnMouseClick(cb func(x, y float64)) {
	c.onClick = cb
}

// DrawRect compiles a geometry quad and sends it to the GPU
func (c *OpenGLCanvas) DrawRect(x, y, w, h float64, hexColor string) {
	if hexColor == "transparent" {
		return
	}

	r, g, b := parseHexColor(hexColor)

	gl.UseProgram(c.shaderProg)

	// Orthographic projection matrix (map logical coordinates to screen space)
	left := float32(0)
	right := float32(c.width)
	bottom := float32(c.height)
	top := float32(0)

	proj := []float32{
		2.0 / (right - left), 0, 0, 0,
		0, 2.0 / (top - bottom), 0, 0,
		0, 0, -1, 0,
		-(right + left) / (right - left), -(top + bottom) / (top - bottom), 0, 1,
	}

	projLoc := gl.GetUniformLocation(c.shaderProg, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projLoc, 1, false, &proj[0])

	colorLoc := gl.GetUniformLocation(c.shaderProg, gl.Str("color\x00"))
	gl.Uniform4f(colorLoc, r, g, b, 1.0)

	// Construct vertex payload (X, Y)
	vertices := []float32{
		float32(x), float32(y),
		float32(x + w), float32(y),
		float32(x + w), float32(y + h),

		float32(x + w), float32(y + h),
		float32(x), float32(y + h),
		float32(x), float32(y),
	}

	gl.BindVertexArray(c.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, c.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 4, gl.FLOAT, false, 4*4, 0)

	// Enable blending for text rendering
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	gl.DrawArrays(gl.TRIANGLES, 0, 6)
}

func (c *OpenGLCanvas) DrawText(x, y float64, text string, fontStr string, size float64) {
	if text == "" {
		return
	}

	// Rasterize text to an RGBA image
	imgWidth := len(text) * 8
	imgHeight := 16
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	// Fill transparent background
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{0, 0, 0, 255}), // Black text
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(0), Y: fixed.I(13)}, // Baseline
	}
	d.DrawString(text)

	// Create OpenGL Texture
	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	// Texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	// Upload image data
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(img.Rect.Size().X), int32(img.Rect.Size().Y),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(img.Pix))

	// Render Texture Quad
	gl.UseProgram(c.shaderProg)

	isTextLoc := gl.GetUniformLocation(c.shaderProg, gl.Str("isText\x00"))
	gl.Uniform1i(isTextLoc, 1)

	// We apply text color using fragment shader
	colorLoc := gl.GetUniformLocation(c.shaderProg, gl.Str("color\x00"))
	gl.Uniform4f(colorLoc, 0.0, 0.0, 0.0, 1.0) // Black

	w := float64(imgWidth)
	h := float64(imgHeight)

	// Y offset adjustment for baseline
	yOffset := y + (size / 2) - (float64(imgHeight) / 2)

	// Construct vertex payload (X, Y, U, V)
	vertices := []float32{
		float32(x), float32(yOffset), 0.0, 0.0,
		float32(x + w), float32(yOffset), 1.0, 0.0,
		float32(x + w), float32(yOffset + h), 1.0, 1.0,

		float32(x + w), float32(yOffset + h), 1.0, 1.0,
		float32(x), float32(yOffset + h), 0.0, 1.0,
		float32(x), float32(yOffset), 0.0, 0.0,
	}

	gl.BindVertexArray(c.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, c.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 4, gl.FLOAT, false, 4*4, 0)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	gl.DrawArrays(gl.TRIANGLES, 0, 6)

	// Clean up texture (in a real engine, we'd cache glyphs in an atlas)
	gl.DeleteTextures(1, &texture)
}

func (c *OpenGLCanvas) DrawImage(x, y float64, data []byte) {
	if len(data) == 0 {
		return
	}

	// Try to decode base64 if it's passed as a string representation
	imgBytes, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		imgBytes = data // Assume raw bytes if decoding fails
	}

	// Decode the image (png/jpeg)
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		log.Printf("Failed to decode image data: %v", err)
		c.DrawRect(x, y, 50, 50, "#CCCCCC") // Fallback
		return
	}

	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	// Convert generic image to RGBA for OpenGL format compatibility
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Create OpenGL Texture
	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	// Texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	// Upload image data
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(imgWidth), int32(imgHeight),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	// Render Texture Quad
	gl.UseProgram(c.shaderProg)

	isTextLoc := gl.GetUniformLocation(c.shaderProg, gl.Str("isText\x00"))
	gl.Uniform1i(isTextLoc, 1) // Reuse the text shader logic which just samples the texture

	// Construct vertex payload (X, Y, U, V)
	w := float64(imgWidth)
	h := float64(imgHeight)

	vertices := []float32{
		float32(x), float32(y), 0.0, 0.0,
		float32(x + w), float32(y), 1.0, 0.0,
		float32(x + w), float32(y + h), 1.0, 1.0,

		float32(x + w), float32(y + h), 1.0, 1.0,
		float32(x), float32(y + h), 0.0, 1.0,
		float32(x), float32(y), 0.0, 0.0,
	}

	gl.BindVertexArray(c.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, c.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 4, gl.FLOAT, false, 4*4, 0)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	gl.DrawArrays(gl.TRIANGLES, 0, 6)

	// Clean up texture (in a real engine, we'd cache images in memory)
	gl.DeleteTextures(1, &texture)
}

func (c *OpenGLCanvas) Flush() error {
	c.window.SwapBuffers()
	glfw.PollEvents()

	// Clear the color buffer for the next frame
	gl.ClearColor(1.0, 1.0, 1.0, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	return nil
}

func (c *OpenGLCanvas) ShouldClose() bool {
	return c.window.ShouldClose()
}

func (c *OpenGLCanvas) Terminate() {
	glfw.Terminate()
}

// --- Utils ---

func compileShader(vertexSrc, fragmentSrc string) (uint32, error) {
	vertexShader, err := createShader(vertexSrc, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	fragmentShader, err := createShader(fragmentSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vertexShader)
	gl.AttachShader(prog, fragmentShader)
	gl.LinkProgram(prog)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		return 0, fmt.Errorf("failed to link shader program")
	}
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return prog, nil
}

func createShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	csource, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csource, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		return 0, fmt.Errorf("failed to compile shader")
	}
	return shader, nil
}

func parseHexColor(hex string) (float32, float32, float32) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0
	}

	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)

	return float32(r) / 255.0, float32(g) / 255.0, float32(b) / 255.0
}
