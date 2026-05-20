using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class ElementControl : MonoBehaviour
{
    public float speed;

    private void Update()
    {
        if (stopped) return;
        transform.position = transform.position + new Vector3(0, 0, speed * Time.deltaTime);
    }

    bool stopped = false;
    public void Stop()
    {
        stopped = true;
    }
}
