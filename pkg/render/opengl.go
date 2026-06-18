package render

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

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
}

const vertexShaderSource = `
#version 330 core
layout(location = 0) in vec2 position;
uniform mat4 projection;
uniform vec4 color;
out vec4 fragColor;
void main() {
    gl_Position = projection * vec4(position, 0.0, 1.0);
    fragColor = color;
}
` + "\x00"

const fragmentShaderSource = `
#version 330 core
in vec4 fragColor;
out vec4 outColor;
void main() {
    outColor = fragColor;
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

	return &OpenGLCanvas{
		window:     window,
		width:      width,
		height:     height,
		shaderProg: prog,
		vao:        vao,
		vbo:        vbo,
	}, nil
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
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 2*4, 0)

	gl.DrawArrays(gl.TRIANGLES, 0, 6)
}

func (c *OpenGLCanvas) DrawText(x, y float64, text string, font string, size float64) {
	// Text rendering is highly complex natively. For Phase 5 Mock, we represent text as a dark bounding box.
	c.DrawRect(x, y, float64(len(text))*10, size, "#333333")
}

func (c *OpenGLCanvas) DrawImage(x, y float64, data []byte) {
	// Mock image
	c.DrawRect(x, y, 50, 50, "#CCCCCC")
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
