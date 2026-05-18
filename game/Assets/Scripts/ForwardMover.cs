using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class ForwardMover : MonoBehaviour
{
    public float min;
    public float max;
    float speed;

    private void Start()
    {
        speed = Random.Range(min, max);
    }

    private void Update()
    {
        transform.position = transform.position + transform.forward * speed * Time.deltaTime;
    }
}
