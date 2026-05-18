Shader "Custom/DepthMask"
{
    SubShader
    {
        Tags { "RenderType"="Opaque" "Queue"="Geometry-10" }
        Pass
        {
            ZWrite On
            ColorMask 0
        }
    }
}